package bilibili

import (
	"encoding/json"
	"testing"
)

// 本文件验证 Bilibili API 响应类型的 JSON 解析正确性。
// B 站 API 契约是逆向来的，字段/tag 一旦对不上整个链路就挂，
// 所以这些解析测试是和真实 API 契约的防线（签名链路靠实测，解析靠这里）。

func TestSearchResponseParsing(t *testing.T) {
	// 真实 B 站 search API 返回结构（wbi search/type）
	body := `{
		"code": 0,
		"message": "0",
		"data": {
			"result": [
				{"bvid":"BV1qD4y1U7fs","title":"【4K】周杰伦《七里香》","author":"音乐私藏馆","duration":"4:58","pic":"//i0.hdslb.com/x.jpg","play":12345},
				{"bvid":"BV1YxgwzzE6H","title":"七里香","author":"如歌如梦","duration":"5:00","pic":"http://i1.hdslb.com/y.jpg","play":678}
			]
		}
	}`
	var resp SearchResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("code 应为 0，got %d", resp.Code)
	}
	if len(resp.Data.Result) != 2 {
		t.Fatalf("应有 2 条结果，got %d", len(resp.Data.Result))
	}
	r0 := resp.Data.Result[0]
	if r0.Bvid != "BV1qD4y1U7fs" {
		t.Errorf("bvid 解析错: %q", r0.Bvid)
	}
	if r0.Author != "音乐私藏馆" {
		t.Errorf("author 解析错: %q", r0.Author)
	}
	// duration 是字符串 "4:58"，验证字段类型
	if r0.Duration != "4:58" {
		t.Errorf("duration 应为字符串 '4:58'，got %q", r0.Duration)
	}
}

func TestViewResponseParsing(t *testing.T) {
	body := `{
		"code": 0,
		"message": "0",
		"data": {
			"aid": 351660720,
			"bvid": "BV1qD4y1U7fs",
			"cid": 987654321,
			"title": "【4K】周杰伦《七里香》",
			"desc": "华语乐坛最美",
			"duration": 298,
			"pic": "http://i0.hdslb.com/cover.jpg",
			"owner": {"name": "音乐私藏馆"}
		}
	}`
	var resp ViewResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("data 不应为 nil")
	}
	if resp.Data.CID != 987654321 {
		t.Errorf("cid 解析错: %d", resp.Data.CID)
	}
	if resp.Data.Owner.Name != "音乐私藏馆" {
		t.Errorf("owner.name 解析错: %q", resp.Data.Owner.Name)
	}
	// duration 是秒（view 接口），区别于 search 的字符串格式
	if resp.Data.Duration != 298 {
		t.Errorf("duration 应为秒数 298，got %d", resp.Data.Duration)
	}
}

func TestPlayURLResponseParsing_Dash(t *testing.T) {
	// playurl 接口 fnval=16 返回 dash 结构（音视频分离）
	body := `{
		"code": 0,
		"message": "0",
		"data": {
			"dash": {
				"audio": [
					{"id":30280,"codecs":"mp4a.40.2","bandwidth":192000,"baseUrl":"https://upos-sz-mirror.example.com/audio1.m4s"},
					{"id":30232,"codecs":"mp4a.40.2","bandwidth":64000,"baseUrl":"https://upos-sz-mirror.example.com/audio2.m4s"}
				],
				"video": []
			}
		}
	}`
	var resp PlayURLResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if resp.Data == nil || resp.Data.Dash == nil {
		t.Fatal("data/dash 不应为 nil")
	}
	if len(resp.Data.Dash.Audio) != 2 {
		t.Fatalf("应有 2 条音频流，got %d", len(resp.Data.Dash.Audio))
	}
	// baseUrl 字段（非 baseURL）——B 站 API 实际返回 baseUrl，确认 tag 对得上
	if resp.Data.Dash.Audio[0].BaseURL != "https://upos-sz-mirror.example.com/audio1.m4s" {
		t.Errorf("audio[0].baseUrl 解析错: %q", resp.Data.Dash.Audio[0].BaseURL)
	}
	// 验证 GetAudioURL 选最高码率的逻辑：192000 > 64000
	best := resp.Data.Dash.Audio[0]
	for _, a := range resp.Data.Dash.Audio[1:] {
		if a.Bandwidth > best.Bandwidth {
			best = a
		}
	}
	if best.Bandwidth != 192000 {
		t.Errorf("应选最高码率 192000，got %d", best.Bandwidth)
	}
}

func TestPlayURLResponseParsing_DURLFallback(t *testing.T) {
	// 部分视频返回 durl 而非 dash（旧格式/未登录降级）
	body := `{
		"code": 0,
		"data": {
			"durl": [{"url":"https://example.com/old-format.flv"}]
		}
	}`
	var resp PlayURLResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(resp.Data.DURL) != 1 {
		t.Fatalf("应有 1 条 durl，got %d", len(resp.Data.DURL))
	}
	if resp.Data.DURL[0].URL != "https://example.com/old-format.flv" {
		t.Errorf("durl[0].url 解析错: %q", resp.Data.DURL[0].URL)
	}
}

func TestRankingResponseParsing(t *testing.T) {
	body := `{
		"code": 0,
		"data": {
			"list": [
				{"aid":1,"bvid":"BV1aaa","title":"热门1","duration":200,"pic":"http://x/1.jpg","owner":{"name":"UP1"},"stat":{"view":99999}}
			]
		}
	}`
	var resp RankingResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(resp.Data.List) != 1 {
		t.Fatalf("应有 1 条排行，got %d", len(resp.Data.List))
	}
	if resp.Data.List[0].Stat.View != 99999 {
		t.Errorf("stat.view 解析错: %d", resp.Data.List[0].Stat.View)
	}
}

// TestParseDuration 验证 B 站 "4:58" 时长字符串 → 毫秒
func TestParseDuration(t *testing.T) {
	cases := []struct{ in, want string }{
		{"4:58", "298000"},     // 4*60+58=298s → 298000ms
		{"1:02:03", "3723000"}, // 1*3600+2*60+3=3723s
		{"0:30", "30000"},
	}
	for _, c := range cases {
		got := parseDuration(c.in)
		if got != atoi2(c.want) {
			t.Errorf("parseDuration(%q) = %d, want %s", c.in, got, c.want)
		}
	}
}

func atoi2(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// TestCleanHTMLTags 验证搜索标题里的 <em> 高亮标签被清除
func TestCleanHTMLTags(t *testing.T) {
	in := "周杰伦<em class=\"keyword\">七里香</em>MV"
	want := "周杰伦七里香MV"
	if got := cleanHTMLTags(in); got != want {
		t.Errorf("cleanHTMLTags = %q, want %q", got, want)
	}
}
