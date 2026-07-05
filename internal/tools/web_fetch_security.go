package tools

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/hwj123hwj/pi-go/internal/util"
)

// web fetch 安全相关的常量（参考 cc-haha WebFetchTool/utils.ts）。
const (
	// maxURLLength URL 长度上限，防超长 URL。
	maxURLLength = 2048
	// maxHTTPContentLength 原始 HTTP 响应体上限（10MB），超了截断，防撑爆内存。
	maxHTTPContentLength = 10 * 1024 * 1024
)

// validateURL 校验 URL 是否合法且安全（参考 cc-haha validateURL）。
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
	if !strings.Contains(hostname, ":") && len(parts) < 2 && hostname != "localhost" {
		return fmt.Errorf("hostname %q 不合法（需至少 2 段或为 localhost）", hostname)
	}

	return nil
}

// isPrivateHost delegates to the shared util.IsPrivateHost for SSRF protection.
func isPrivateHost(hostname string) bool {
	return util.IsPrivateHost(hostname)
}

// isPrivateIP delegates to the shared util.IsPrivateIP.
func isPrivateIP(ip net.IP) bool {
	return util.IsPrivateIP(ip)
}
