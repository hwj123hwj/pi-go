package bilibili

import (
	"testing"
)

// helper：从标题列表构造 VideoResult
func resultsFromTitles(titles ...string) []VideoResult {
	out := make([]VideoResult, len(titles))
	for i, title := range titles {
		out[i] = VideoResult{Bvid: title, Title: title}
	}
	return out
}

// helper：断言过滤后剩下的标题集合
func assertRemain(t *testing.T, query string, in []string, wantRemain []string) {
	t.Helper()
	got := filterQualityResults(query, resultsFromTitles(in...))
	gotTitles := make([]string, len(got))
	for i, r := range got {
		gotTitles[i] = r.Title
	}
	if len(gotTitles) != len(wantRemain) {
		t.Errorf("query=%q\n输入: %v\n期望保留: %v\n实际保留: %v", query, in, wantRemain, gotTitles)
		return
	}
	wantSet := make(map[string]bool, len(wantRemain))
	for _, w := range wantRemain {
		wantSet[w] = true
	}
	for _, g := range gotTitles {
		if !wantSet[g] {
			t.Errorf("query=%q 意外保留: %q\n输入: %v\n期望保留: %v\n实际: %v", query, g, in, wantRemain, gotTitles)
		}
	}
}

// 用例：鼓谱教学必须剔除（Taylor Swift 真实 case）
func TestFilter_Blacklist_DrumScore(t *testing.T) {
	assertRemain(t, "love story",
		[]string{"Love Story【Taylor Swift】动态鼓谱教学", "Love Story - Taylor Swift 官方MV"},
		[]string{"Love Story - Taylor Swift 官方MV"})
}

// 用例：reaction 必须剔除
func TestFilter_Blacklist_Reaction(t *testing.T) {
	assertRemain(t, "七里香",
		[]string{"老外首次反应周杰伦《七里香》reaction", "周杰伦《七里香》MV"},
		[]string{"周杰伦《七里香》MV"})
}

// 用例：钢琴谱/教学必须剔除
func TestFilter_Blacklist_PianoScore(t *testing.T) {
	assertRemain(t, "晴天",
		[]string{"晴天 钢琴谱简谱教学", "【4K修复】周杰伦-晴天MV"},
		[]string{"【4K修复】周杰伦-晴天MV"})
}

// 用例：混剪/串烧必须剔除
func TestFilter_Blacklist_Medley(t *testing.T) {
	assertRemain(t, "富士山下",
		[]string{"陈奕迅经典串烧medley混剪", "陈奕迅《富士山下》"},
		[]string{"陈奕迅《富士山下》"})
}

// 用例：Hi-res 重制必须保留（用户认可，不算翻唱/非原版）
func TestFilter_Keep_HiRes(t *testing.T) {
	assertRemain(t, "富士山下",
		[]string{"百万录音棚大声听陈奕迅《富士山下》Hi-res"},
		[]string{"百万录音棚大声听陈奕迅《富士山下》Hi-res"})
}

// 用例：4K 修复 MV 必须保留
func TestFilter_Keep_4KMV(t *testing.T) {
	assertRemain(t, "晴天",
		[]string{"【4K修复】周杰伦-晴天MV"},
		[]string{"【4K修复】周杰伦-晴天MV"})
}

// 用例：英文短语整体匹配（不拆单字）—— love story 的鼓谱应剔除
func TestFilter_EnglishPhraseNotSplit(t *testing.T) {
	// "love story 鼓谱"：候选 ["love story"]，含 love story 过同名，但含"鼓谱"→ 剔除
	assertRemain(t, "love story",
		[]string{"Love Story 鼓谱", "Love Story Official Video"},
		[]string{"Love Story Official Video"})
}

// 用例：书名号歌名保留
func TestFilter_Keep_TitleInBookQuotes(t *testing.T) {
	assertRemain(t, "周杰伦 七里香",
		[]string{"周杰伦《七里香》官方MV"},
		[]string{"周杰伦《七里香》官方MV"})
}

// 用例：同名过滤（标题无歌名 → 剔除）
func TestFilter_SameName_NoSongName(t *testing.T) {
	assertRemain(t, "七里香",
		[]string{"周杰伦2024演唱会全程", "七里香 现场版"},
		[]string{"七里香 现场版"})
}

// 用例：兜底 Pass1 空则放宽同名（非黑名单但无歌名 → Pass2 保留）
func TestFilter_Fallback_Pass1Empty(t *testing.T) {
	// "周杰伦2024演唱会全程"：无"七里香"→ Pass1 剔除；非黑名单 → Pass2 保留
	assertRemain(t, "七里香",
		[]string{"周杰伦2024演唱会全程"},
		[]string{"周杰伦2024演唱会全程"})
}

// 用例：全黑名单 → 返回空（不回退到不过滤）
func TestFilter_AllBlacklist_Empty(t *testing.T) {
	got := filterQualityResults("七里香", resultsFromTitles("七里香 鼓谱", "七里香 reaction"))
	if len(got) != 0 {
		t.Errorf("全黑名单应返回空，got %d 条: %v", len(got), got)
	}
}

// 用例：空输入返回空
func TestFilter_EmptyInput(t *testing.T) {
	got := filterQualityResults("七里香", nil)
	if len(got) != 0 {
		t.Errorf("空输入应返回空，got %d", len(got))
	}
}

// 黑名单词表正向（应命中剔除）
func TestBlacklist_Hits(t *testing.T) {
	for _, w := range []string{"动态鼓谱", "鼓谱教学", "钢琴谱", "简谱", "首次反应", "reaction", "听后感", "串烧", "medley", "混剪"} {
		if passesBlacklist("某歌 " + w) {
			t.Errorf("黑名单词 %q 应被命中剔除（passesBlacklist 应返回 false）", w)
		}
	}
}

// 黑名单词表反向（这些不应命中：Hi-res/4K/MV 等是正常修饰）
func TestBlacklist_NoFalsePositive(t *testing.T) {
	for _, title := range []string{
		"【4K修复】周杰伦-晴天MV",
		"百万录音棚大声听陈奕迅《富士山下》Hi-res",
		"周杰伦《七里香》官方MV",
		"周杰伦 七里香 现场版 Live",
		"重制版 remaster",
	} {
		if !passesBlacklist(title) {
			t.Errorf("正常标题误判为黑名单: %q", title)
		}
	}
}

// extractSongNameCandidates：中文 OR 切分
func TestExtractCandidates_Chinese(t *testing.T) {
	got := extractSongNameCandidates("周杰伦 七里香")
	want := map[string]bool{"周杰伦": true, "七里香": true}
	if len(got) != 2 {
		t.Fatalf("应切出 2 个候选，got %v", got)
	}
	for _, c := range got {
		if !want[c] {
			t.Errorf("意外候选 %q", c)
		}
	}
}

// extractSongNameCandidates：英文不拆单字
func TestExtractCandidates_EnglishPhrase(t *testing.T) {
	got := extractSongNameCandidates("love story")
	if len(got) != 1 || got[0] != "love story" {
		t.Errorf("love story 应整体作为一个候选，got %v", got)
	}
}

// extractSongNameCandidates：单字中文不进候选（避免"的"等误匹配）
func TestExtractCandidates_SingleChineseRune(t *testing.T) {
	got := extractSongNameCandidates("啊")
	if len(got) != 0 {
		t.Errorf("单字中文不应进候选（兜底走整个query），got %v", got)
	}
}
