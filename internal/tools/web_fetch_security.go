package tools

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// web fetch 安全相关的常量（参考 cc-haha WebFetchTool/utils.ts）。
const (
	// maxURLLength URL 长度上限，防超长 URL。
	maxURLLength = 2048
	// maxHTTPContentLength 原始 HTTP 响应体上限（10MB），超了截断，防撑爆内存。
	maxHTTPContentLength = 10 * 1024 * 1024
)

// validateURL 校验 URL 是否合法且安全（参考 cc-haha validateURL）。
// 规则：
//   - 长度不超过 maxURLLength
//   - 协议必须是 http/https（http 后续请求时会升级 https）
//   - 禁止带 username/password 的 URL（防凭证泄露/内网伪装）
//   - hostname 至少 2 段（过滤 "localhost"/"intranet" 等裸主机名）
func validateURL(rawURL string) error {
	if len(rawURL) > maxURLLength {
		return fmt.Errorf("URL 过长（>%d 字符）", maxURLLength)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL 解析失败: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("仅支持 http/https 协议，得到 %q", parsed.Scheme)
	}

	// 禁止带凭证的 URL（如 http://user:pass@host）
	if parsed.User != nil && (parsed.User.Username() != "" || strings.Contains(rawURL, "@")) {
		return fmt.Errorf("禁止带用户名/密码的 URL")
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL 缺少 hostname")
	}
	parts := strings.Split(hostname, ".")
	// IPv6 已被 Hostname() 去掉方括号；纯 IP 的 hostname 没有 "."，靠 isPrivateHost 兜底
	if !strings.Contains(hostname, ":") && len(parts) < 2 && hostname != "localhost" {
		// 非localhost 的裸主机名（如 "intranet"）要求至少 2 段
		// localhost 单独放行，由 isPrivateHost 统一拦截
		return fmt.Errorf("hostname %q 不合法（需至少 2 段或为 localhost）", hostname)
	}

	return nil
}

// isPrivateHost 判断 hostname 是否指向内网/保留地址（SSRF 防护）。
// 参考DeepV isPrivateIp + 补充 localhost、IPv6、云元数据地址（169.254.169.254）。
//
// 处理顺序：
//  1. localhost / *.local 等保留名直接拦截
//  2. 若 hostname 是 IP 字面量，直接判断
//  3. 否则 DNS 解析后判断解析到的 IP（防域名指向内网）
func isPrivateHost(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return true
	}
	if strings.HasSuffix(hostname, ".local") {
		return true
	}

	// 直接是 IP 字面量
	if ip := net.ParseIP(hostname); ip != nil {
		return isPrivateIP(ip)
	}

	// 域名：DNS 解析后判断（任一解析结果是内网即拦截）
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// 解析失败保守放行（真实域名解析失败由 HTTP 层报错）；不在此阻断
		return false
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return true
		}
	}
	return false
}

// isPrivateIP 判断 IP 是否属于内网/保留/回环/链路本地等不可达公网的地址段。
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	// 回环、私网、链路本地、未指定、组播、保留
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// 云元数据服务地址（AWS/GCP/Azure 等）：169.254.169.254（已被 IsLinkLocalUnicast 覆盖，
	// 但显式列出避免 Go 版本差异）
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 169 && v4[1] == 254 && v4[2] == 169 && v4[3] == 254 {
			return true
		}
	}
	return false
}
