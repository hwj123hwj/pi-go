package bilibili

import (
	"strings"
	"unicode"
)

// 本文件对 B站搜索结果做质量过滤，剔除"明显不是在放这首歌"的视频。
// 解决：搜"Love Story"首条可能是鼓谱教学、搜"七里香"首条可能是 reaction。
// 由 Client.Search 在 return 前调用，让所有上层（adapter/playByQuery/跨源兜底）自动受益。

// blacklist 是标题子串命中即剔除的词。黑名单是硬规则，永不放宽。
// 不含 cover/翻唱/学唱 —— 与 Hi-res 重制语义边界模糊，误杀风险 > 收益；
// 不含 伴奏 —— 原唱伴奏带(KTV版)是合法可播源。
var blacklist = []string{
	// 教学/谱面类（不是在放原唱，是在教/在演奏谱子）
	"教学", "教程", "教学版", "鼓谱", "动态鼓谱", "鼓机", "架子鼓谱",
	"钢琴谱", "吉他谱", "简谱", "五线谱", "谱面", "跟练",
	// reaction/解说类（不是放歌，是看别人放歌的反应）
	"reaction", "react", "首次反应", "听后感", "听歌反应", "影院反应",
	// 合集/混剪类（歌名只匹配其中一首，不是完整单曲）
	"合集", "串烧", "medley", "混剪", "混剪版", "mashup",
}

// filterQualityResults 对 B站搜索结果做两道闸门，返回通过的质量合格结果。
//
//	闸门1（黑名单）：标题命中黑名单词 → 剔除（硬规则）
//	闸门2（同名）：歌名核心词须出现在标题里（软校验，OR 关系）
//
// 两段式兜底：Pass1(黑名单∩同名)全空 → 放宽为仅黑名单（Pass2）。
// 黑名单永不放宽——即使全被杀也不回退到不过滤，播教学/reaction 必错，宁可返回空。
func filterQualityResults(query string, results []VideoResult) []VideoResult {
	if len(results) == 0 {
		return results
	}
	pass1 := make([]VideoResult, 0, len(results))
	for _, r := range results {
		if passesBlacklist(r.Title) && passesSameName(query, r.Title) {
			pass1 = append(pass1, r)
		}
	}
	if len(pass1) > 0 {
		return pass1
	}
	// Pass1 空：同名过滤可能太严（歌名被翻译/缩写/书名号包围），放宽只过黑名单。
	pass2 := make([]VideoResult, 0, len(results))
	for _, r := range results {
		if passesBlacklist(r.Title) {
			pass2 = append(pass2, r)
		}
	}
	return pass2
}

// passesBlacklist 判断标题是否避开所有黑名单词。命中任一黑名单词 → false。
func passesBlacklist(title string) bool {
	t := lightNormalize(title)
	for _, w := range blacklist {
		if strings.Contains(t, strings.ToLower(w)) {
			return false
		}
	}
	return true
}

// passesSameName 判断标题是否包含 query 提取的歌名候选之一（OR 关系）。
// query 提取不到有效候选（全标点/单字符）→ 退化为"标题含整个 query 归一化"，宁可漏过不全杀。
func passesSameName(query, title string) bool {
	candidates := extractSongNameCandidates(query)
	t := lightNormalize(title)
	if len(candidates) == 0 {
		// 兜底：query 本身归一化后作为唯一候选
		return strings.Contains(t, lightNormalize(query))
	}
	for _, c := range candidates {
		if c != "" && strings.Contains(t, c) {
			return true
		}
	}
	return false
}

// extractSongNameCandidates 从 query 提取歌名候选 token。
//
//	英文：含拉丁字母的连续段保留整个短语（含内部空格）作为一个 token（绝不拆单字！）
//	      —— 否则 "love story" 拆成 love/story 会让任何含"love"的标题都过
//	中文：按非拉丁分隔符切，token 长度 ≥2（rune 数）才进候选
//	多候选是 OR 关系：query="周杰伦 七里香" → ["周杰伦","七里香"]，命中其一即过
func extractSongNameCandidates(query string) []string {
	nq := strings.ToLower(strings.TrimSpace(query))
	if nq == "" {
		return nil
	}

	var candidates []string
	var seg []rune
	flushSeg := func() {
		if len(seg) == 0 {
			return
		}
		s := strings.TrimSpace(string(seg))
		seg = seg[:0]
		if s == "" {
			return
		}
		if isLatin(s) {
			// 英文段：整体作候选（可能含空格，如 "love story"）
			candidates = append(candidates, s)
		} else if len([]rune(s)) >= 2 {
			// 中文段：≥2 rune 才算有效候选（避免单字误匹配）
			candidates = append(candidates, s)
		}
	}

	for _, r := range nq {
		// 标点（非空格）→ 切段
		if !unicode.IsSpace(r) && isSeparator(r) {
			flushSeg()
			continue
		}
		// 空格：若当前段非空且含非拉丁（中文）→ 空格是切分点；否则（拉丁段或空段）→ 并入段
		if unicode.IsSpace(r) {
			if len(seg) > 0 && !segIsLatin(seg) {
				flushSeg()
				continue
			}
			// 拉丁段内的空格：并入（保留 "love story" 整体）
			seg = append(seg, r)
			continue
		}
		seg = append(seg, r)
	}
	flushSeg()
	return candidates
}

// segIsLatin 判断已累积的 rune 段是否当前全是 ASCII 拉丁字母（含内部空格）。
// 用于决定空格是"拉丁短语内连接"还是"中文段切分点"。
func segIsLatin(seg []rune) bool {
	if len(seg) == 0 {
		return false
	}
	for _, r := range seg {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == ' ') {
			return false
		}
	}
	return true
}

// isLatin 判断字符串是否全是 ASCII 拉丁字母（含空格，用于候选分类）。
func isLatin(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == ' ') {
			return false
		}
	}
	return true
}

// isSeparator 判断 query 里的分隔标点（不含空格，空格单独处理见 extractSongNameCandidates）。
func isSeparator(r rune) bool {
	switch r {
	case '\t', ',', '，', '、', '|', '/', '\\', '-', '—', '–', '·', '・', '：', ':',
		'(', ')', '（', '）', '《', '》', '【', '】',
		'"', '\'', '`', // 半角引号
		'“', '”', // 全角引号 “ ”
		'‘', '’': // 全角单引号 ‘ ’
		return true
	}
	return false
}

// lightNormalize 轻归一化标题，供黑名单匹配和同名匹配使用。
//
//	只做：小写化 + 去掉首尾空白
//	**不去括号段**！否则"七里香（教学版）"会被归一成"七里香"，黑名单词"教学"丢失，闸门1失效。
//
// 注：标题里的 <em> 高亮标签已由 cleanHTMLTags（search.go）在解析时清除，这里不重复处理。
func lightNormalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
