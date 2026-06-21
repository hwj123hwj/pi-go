package netease

import "testing"

func TestNormalizeSongName(t *testing.T) {
	cases := []struct{ in, want string }{
		// 基础
		{"七里香", "七里香"},
		{"晴天", "晴天"},
		{"", ""},
		// 半角括号段
		{"七里香 (Live)", "七里香"},
		{"七里香 (钢琴版)", "七里香"},
		{"七里香 (acoustic)", "七里香"},
		// 全角括号段
		{"七里香（Live）", "七里香"},
		{"七里香（钢琴版）", "七里香"},
		// 多段括号
		{"七里香 (Live) (钢琴版)", "七里香"},
		// " - " 分隔的已知版本后缀
		{"七里香 - 现场版", "七里香"},
		{"七里香-live", "七里香"},
		{"七里香 - 纯音乐", "七里香"},
		{"七里香 - 周杰伦", "七里香 - 周杰伦"}, // "周杰伦"非版本词 → 保留 → 不同名
		// 拉丁大小写 + 空白折叠
		{"  Love   Story  ", "love story"},
		{"Love Story (Taylor's Version)", "love story"},
		// 全角空格折叠（width 未做，但 unicode.IsSpace 认全角空格）
		{"晴天　(Live)", "晴天"}, // 　 全角空格
	}
	for _, c := range cases {
		got := NormalizeSongName(c.in)
		if got != c.want {
			t.Errorf("NormalizeSongName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSameSongName(t *testing.T) {
	if !SameSongName("七里香", "七里香 (Live)") {
		t.Error("七里香 vs 七里香 (Live) 应判定同名")
	}
	if !SameSongName("七里香 (钢琴版)", "七里香") {
		t.Error("七里香 (钢琴版) vs 七里香 应判定同名")
	}
	if SameSongName("七里香", "夜曲") {
		t.Error("七里香 vs 夜曲 不应同名")
	}
	// bug 核心场景：同名翻唱不应被判为"同一首歌的合法替代"以外的同名——
	// 这里验证的是归一化层面"七里香"与"七里香 - 周杰伦"不等（防误归一化）
	if SameSongName("七里香", "七里香 - 周杰伦") {
		t.Error("七里香 vs 七里香 - 周杰伦 不应同名（防误归一化）")
	}
}

func TestJoinArtists(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"周杰伦"}, "周杰伦"},
		{[]string{"周杰伦", "费玉清"}, "周杰伦 / 费玉清"},
		{[]string{"周杰伦", " ", "费玉清"}, "周杰伦 / 费玉清"}, // 去空白项
		{[]string{"  周杰伦  "}, "周杰伦"}, // trim
	}
	for _, c := range cases {
		got := JoinArtists(c.in)
		if got != c.want {
			t.Errorf("JoinArtists(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSameArtist(t *testing.T) {
	// 单歌手：候选与基准一致
	if !SameArtist([]string{"周杰伦"}, []string{"周杰伦"}, "") {
		t.Error("单歌手一致应匹配")
	}
	// 合唱：候选是基准的子集（周杰伦在两者中）
	if !SameArtist([]string{"周杰伦", "费玉清"}, []string{"周杰伦"}, "") {
		t.Error("合唱含基准歌手应匹配")
	}
	// 合唱：搜"费玉清 千里之外"，基准含费玉清，候选也含 → match（修合唱丢失）
	if !SameArtist([]string{"周杰伦", "费玉清"}, []string{"费玉清"}, "") {
		t.Error("合唱按费玉清基准应匹配")
	}
	// 不同歌手
	if SameArtist([]string{"张学友"}, []string{"周杰伦"}, "") {
		t.Error("不同歌手不应匹配")
	}
	// query 兜底：基准无歌手，但 query 含候选歌手
	if !SameArtist([]string{"周杰伦"}, nil, "周杰伦 七里香") {
		t.Error("query 含候选歌手应兜底匹配")
	}
	// 大小写不敏感
	if !SameArtist([]string{"Jay Chou"}, []string{"jay chou"}, "") {
		t.Error("歌手大小写应不敏感")
	}
	// 全空 → 不匹配（由调用方决定退化）
	if SameArtist(nil, nil, "") {
		t.Error("全空歌手不应匹配")
	}
}

func TestIntendedArtists(t *testing.T) {
	candidates := [][]string{
		{"学员翻唱"},
		{"周杰伦"},
		{"周杰伦", "费玉清"},
	}
	// query 含"周杰伦" → 识别出周杰伦（去重，合唱里的也算一次）
	got := IntendedArtists("周杰伦 七里香", candidates)
	if len(got) != 1 || got[0] != "周杰伦" {
		t.Errorf("query='周杰伦 七里香' 应识别 [周杰伦]，got %v", got)
	}
	// query 含合唱两歌手
	got = IntendedArtists("周杰伦 费玉清 千里之外", candidates)
	if len(got) != 2 {
		t.Errorf("应识别 2 个歌手，got %v", got)
	}
	// 纯歌名 query → 提取不到 → nil（调用方退化用基准首条）
	got = IntendedArtists("卡农", candidates)
	if got != nil {
		t.Errorf("纯歌名应返回 nil，got %v", got)
	}
	// 空 query → nil
	if IntendedArtists("", candidates) != nil {
		t.Error("空 query 应返回 nil")
	}
}
