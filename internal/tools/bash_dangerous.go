package tools

import (
	"regexp"
	"strings"
)

// whyDangerous 检查 shell 命令是否具有破坏性，返回命中规则的人类可读描述。
// 返回空串表示安全（无需确认）。
//
// 判定原则：保守但不打扰。只拦截真正可能造成不可逆损失的操作
// （删数据、覆盖文件、提权、远程脚本、磁盘/系统操作、破坏 git 历史）。
// 普通读操作和常规命令直接放行。
func whyDangerous(cmd string) string {
	// 规范化：压缩连续空白，便于正则匹配跨行/多空格命令
	normalized := strings.Join(strings.Fields(cmd), " ")

	for _, rule := range dangerousRules {
		if rule.pattern.MatchString(normalized) {
			return rule.reason
		}
	}
	return ""
}

type dangerousRule struct {
	pattern *regexp.Regexp
	reason  string
}

// dangerousRules 按从具体到通用排列。正则匹配规范化后的命令（空白已压缩）。
var dangerousRules = []dangerousRule{
	// 递归/强删文件
	{regexp.MustCompile(`(?i)\brm\s+(-[a-z]*r[a-z]*\s+|--recursive\s+)`), "递归删除"},
	{regexp.MustCompile(`(?i)\brm\s+(-[a-z]*f[a-z]*\s+)?(-[a-z]*r|--recursive)`), "递归删除"},
	{regexp.MustCompile(`(?i)\brm\s+-[a-z]*[rf][a-z]*\s+/(\s|$)`), "删除根目录或绝对路径"},

	// 覆盖文件（单 > 重定向）。">>" 追加相对安全，不拦。
	// [^>]>[^>]: 前后字符都不是 >，可正确排除 >>（追加）。
	{regexp.MustCompile(`[^>]>[^>]`), "覆盖重定向写文件"},
	// 命令开头的 > （截断文件），如 "> file" 或 ">file"。
	{regexp.MustCompile(`^\s*>[^>]`), "覆盖重定向写文件"},
	{regexp.MustCompile(`\|\s*\bsudo\b`), "通过管道提权执行"},

	// 提权
	{regexp.MustCompile(`(?i)\bsudo\b`), "sudo 提权"},
	{regexp.MustCompile(`(?i)\bdoas\b`), "doas 提权"},

	// 递归权限变更
	{regexp.MustCompile(`(?i)\bchmod\s+(-[a-z]*r[a-z]*\s+|--recursive\s+)`), "递归改权限"},
	{regexp.MustCompile(`(?i)\bchown\s+(-[a-z]*r[a-z]*\s+|--recursive\s+)`), "递归改属主"},

	// 磁盘/分区/设备
	{regexp.MustCompile(`(?i)\bdd\b\s+if=`), "dd 磁盘读写"},
	{regexp.MustCompile(`(?i)\b(mkfs|fdisk|parted|wipefs)\b`), "磁盘格式化/分区操作"},

	// 远程脚本执行（curl/wget 管道到 shell）
	{regexp.MustCompile(`(?i)\b(curl|wget)\b.*\|\s*(sh|bash|zsh|fish)`), "远程脚本直接执行"},

	// 系统电源/进程
	{regexp.MustCompile(`(?i)\b(shutdown|reboot|halt|poweroff|init\s+0)\b`), "关机/重启"},
	{regexp.MustCompile(`(?i)\bkill(?:all)?\s+-9\b`), "强杀进程 (kill -9)"},

	// 破坏 git 历史
	{regexp.MustCompile(`(?i)\bgit\s+push\b.*--force`), "git 强制推送"},
	{regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`), "git 硬重置（丢弃改动）"},
	{regexp.MustCompile(`(?i)\bgit\s+clean\s+-[a-z]*[fd]`), "git 清理未跟踪文件"},
}
