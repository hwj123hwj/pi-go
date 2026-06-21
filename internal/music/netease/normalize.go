package netease

import (
	"strings"
	"unicode"
)

// 本文件提供"同一首歌"判断所需的归一化与匹配纯函数。
// 用于 music_play 自动跳过 VIP 时，校验候选结果与基准（搜索首条）是否同名、歌手是否匹配，
// 避免把同名翻唱/不同曲当作原版播放。

// versionSuffixWords 是 " - " 分隔后允许剥离的已知版本/编曲词。
// 用白名单而非全剥：网易云歌名里"歌名 - 歌手"极少见（歌手在 artists 字段），
// 但"歌名 - Live"常见。宁可漏归一化（漏了靠歌手匹配兜底），不可误归一化（误了就播错歌）。
var versionSuffixWords = []string{
	"live", "现场版", "现场", "演唱会",
	"acoustic", "吉他版", "钢琴版", "纯音乐", "伴奏", "instrumental",
	"remix", "dj版", "cover", "翻唱", "karaoke",
	"remaster", "remastered", "重制版",
}

// NormalizeSongName 归一化歌名，用于"同名"判断。
//
// pipeline：去括号段(全/半角) → 剥已知版本后缀 → 折叠空白 → 小写化。
// 例：七里香 / 七里香 (Live) / 七里香（钢琴版） / 七里香 - 现场版 → 七里香
//
// 七里香 - 周杰伦 不变（"周杰伦"非版本词）→ 与基准不等 → 视为不同名。
func NormalizeSongName(name string) string {
	if name == "" {
		return ""
	}

	n := stripBracketSegments(name) // 去全/半角括号段
	n = stripVersionSuffix(n)       // 剥已知版本后缀
	n = collapseSpaces(n)           // 折叠空白
	n = strings.ToLower(n)          // 小写化（拉丁字母；中文无影响）
	return strings.TrimSpace(n)
}

// SameSongName 判断两歌名归一化后是否相等。
func SameSongName(a, b string) bool {
	return NormalizeSongName(a) == NormalizeSongName(b)
}

// JoinArtists 把歌手列表拼成展示字符串：空→""，单→该名，多→" / " 拼接。
// 用于 Song.Artist 字段的统一构造（向后兼容：消费方仍读 Artist 字符串）。
func JoinArtists(names []string) string {
	var clean []string
	for _, n := range names {
		if t := strings.TrimSpace(n); t != "" {
			clean = append(clean, t)
		}
	}
	switch len(clean) {
	case 0:
		return ""
	case 1:
		return clean[0]
	default:
		return strings.Join(clean, " / ")
	}
}

// SameArtist 判断候选歌手是否匹配"基准意图"。
//
// 匹配条件：候选任一歌手 ∈ 基准歌手集合，或候选任一歌手出现在 query 文本里（query 兜底，处理纯歌名基准无歌手时仍按 query 推断歌手）。
// 三方（候选/基准/query）都无有效歌手 → 返回 false（由调用方决定是否退化）。
func SameArtist(trackArtists, baseArtists []string, query string) bool {
	trackSet := normalizeSet(trackArtists)
	baseSet := normalizeSet(baseArtists)

	// 基准歌手集合命中候选任一歌手
	for a := range trackSet {
		if baseSet[a] {
			return true
		}
	}

	// query 兜底：query 归一化文本包含某候选歌手
	if query != "" {
		nq := NormalizeSongName(query) // 复用归一化（小写、去空白）
		if nq != "" {
			for a := range trackSet {
				if a != "" && strings.Contains(nq, a) {
					return true
				}
			}
		}
	}

	return false
}

// IntendedArtists 从 query 文本中识别"用户想要的歌手"。
//
// 在所有候选歌手里，挑出名字出现在 query 文本中的作为 intendedArtists。
// 用于歌手匹配基准：query="周杰伦 七里香" + 候选含"周杰伦" → intendedArtists=[周杰伦]，
// 这样翻唱（学员翻唱）对不上 intendedArtists → 走 tier2 标注，不会把翻唱当原版。
//
// query 提取不到任何歌手（纯歌名 query）→ 返回 nil，由调用方退化用基准首条歌手。
func IntendedArtists(query string, candidateArtists [][]string) []string {
	if query == "" {
		return nil
	}
	nq := NormalizeSongName(query)
	if nq == "" {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, arts := range candidateArtists {
		for _, a := range arts {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			na := strings.ToLower(a)
			if strings.Contains(nq, na) && !seen[na] {
				seen[na] = true
				out = append(out, a)
			}
		}
	}
	return out
}

// normalizeSet 把歌手列表转成归一化后的 set（小写、去空白、去空）。
func normalizeSet(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		k := strings.ToLower(strings.TrimSpace(n))
		if k != "" {
			m[k] = true
		}
	}
	return m
}

// stripBracketSegments 去除所有括号段：(…)、[…]、{…}（含全角）。
func stripBracketSegments(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '(', '（', '[', '【', '{':
			depth++
		case ')', '）', ']', '】', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// stripVersionSuffix 去除结尾的已知版本后缀。
// 形如 "歌名 - live"、"歌名-live"、"歌名 live" → "歌名"。
// 只剥离一个匹配的后缀，且必须贴近结尾（允许尾部空白）。
func stripVersionSuffix(s string) string {
	s = strings.TrimSpace(s)
	lowered := strings.ToLower(s)
	for _, w := range versionSuffixWords {
		for _, sep := range []string{" - ", "-", " ", "—", "–"} {
			suffix := sep + w
			if strings.HasSuffix(lowered, suffix) {
				return strings.TrimSpace(s[:len(s)-len(suffix)])
			}
		}
	}
	return s
}

// collapseSpaces 把连续空白压成单空格。
func collapseSpaces(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
}
