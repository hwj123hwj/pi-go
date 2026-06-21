package music

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/earendil-works/pi-go/internal/music/netease"
)

// testRouter creates a minimal SourceRouter with a netease adapter for testing.
func testRouter() *SourceRouter {
	return NewSourceRouter(SourceNetease, NewNetEaseAdapter(netease.NewClient()))
}

// fakeAudioUpstream 起一个模拟上游 CDN 的 server：
// 记录收到的 Range 头，并按 Range 返回对应的字节范围（206）或全量（200）。
// 用最小实现验证 handler 的 Range 透传，不模拟真实 CDN 的边界细节。
func fakeAudioUpstream(t *testing.T, content []byte) (*httptest.Server, *string) {
	t.Helper()
	var gotRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")

		// 支持简单的 "bytes=start-end" 解析（够测试用）
		if gotRange != "" {
			var start, end int
			_, _ = parseRangeSimple(gotRange, len(content), &start, &end)
			w.Header().Set("Content-Range", formatContentRange(start, end, len(content)))
			w.Header().Set("Content-Length", itoa(int64(end-start+1)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(content[start : end+1])
			return
		}

		// 全量请求
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", itoa(int64(len(content))))
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	t.Cleanup(srv.Close)
	return srv, &gotRange
}

// fakeErrorUpstream 起一个永远返回 403 的 server（模拟 CDN 防盗链失效）。
func fakeErrorUpstream(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte("forbidden"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestHandleAudio_ErrorNotCached 验证上游错误响应（403）不被缓存——
// 否则防盗链失效/下架会被缓存 24h，导致歌曲"永久"播不了。
func TestHandleAudio_ErrorNotCached(t *testing.T) {
	upstream := fakeErrorUpstream(t, http.StatusForbidden)

	cache := NewCache()
	cache.Set(AudioKey("netease", "99"), upstream.URL+"/audio.mp3", TTLAudio)
	h := NewHandler(testRouter(), cache)

	req := httptest.NewRequest(http.MethodGet, "/music/audio/99", nil)
	req.SetPathValue("song_id", "99")
	rec := httptest.NewRecorder()
	h.handleAudio(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// 状态码透传（403 不该被 handler 吞掉或改写）
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (passthrough)", resp.StatusCode)
	}
	// 错误响应必须 no-store，不可缓存
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q (error must not be cached)", cc, "no-store")
	}
}

// TestHandleAudio_ForwardsRangeHeader 验证客户端的 Range 头被透传到上游，
// 且上游返回的 206 + Content-Range 被原样转发给客户端（seek 支持的核心）。
func TestHandleAudio_ForwardsRangeHeader(t *testing.T) {
	// 1000 字节的假音频
	content := make([]byte, 1000)
	for i := range content {
		content[i] = byte(i % 256)
	}
	upstream, gotRange := fakeAudioUpstream(t, content)

	// 预填 cache：让 handler 直接命中缓存，指向测试 upstream
	cache := NewCache()
	cache.Set(AudioKey("netease", "42"), upstream.URL+"/audio.mp3", TTLAudio)
	h := NewHandler(testRouter(), cache)

	// 客户端请求 bytes=100-199（100 字节）
	req := httptest.NewRequest(http.MethodGet, "/music/audio/42", nil)
	req.SetPathValue("song_id", "42")
	req.Header.Set("Range", "bytes=100-199")
	rec := httptest.NewRecorder()

	h.handleAudio(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// 1. Range 头透传到上游
	if *gotRange != "bytes=100-199" {
		t.Fatalf("upstream Range = %q, want %q", *gotRange, "bytes=100-199")
	}
	// 2. 206 状态码透传
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", resp.StatusCode)
	}
	// 3. Content-Range 透传
	if cr := resp.Header.Get("Content-Range"); cr != "bytes 100-199/1000" {
		t.Fatalf("Content-Range = %q, want %q", cr, "bytes 100-199/1000")
	}
	// 4. Accept-Ranges 存在
	if ar := resp.Header.Get("Accept-Ranges"); ar != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want %q", ar, "bytes")
	}
	// 4b. 成功响应（206）应被缓存
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=86400" {
		t.Fatalf("Cache-Control = %q, want cached 24h", cc)
	}
	// 5. 返回的是请求范围（100 字节）
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 100 {
		t.Fatalf("body length = %d, want 100", len(body))
	}
	// 6. 内容正确（content[100..199]）
	for i, b := range body {
		if b != content[100+i] {
			t.Fatalf("body[%d] = %d, want %d", i, b, content[100+i])
		}
	}
}

// TestHandleAudio_NoRangeReturnsFullContent 验证无 Range 头时返回 200 全量。
func TestHandleAudio_NoRangeReturnsFullContent(t *testing.T) {
	content := []byte("hello-audio-full-content")
	upstream, gotRange := fakeAudioUpstream(t, content)

	cache := NewCache()
	cache.Set(AudioKey("netease", "7"), upstream.URL+"/audio.mp3", TTLAudio)
	h := NewHandler(testRouter(), cache)

	req := httptest.NewRequest(http.MethodGet, "/music/audio/7", nil)
	req.SetPathValue("song_id", "7")
	rec := httptest.NewRecorder()
	h.handleAudio(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if *gotRange != "" {
		t.Fatalf("upstream Range = %q, want empty (no Range forwarded)", *gotRange)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(content) {
		t.Fatalf("body mismatch")
	}
}

// ── 最小化的 Range 解析/格式化辅助（仅供测试用，不模拟完整 HTTP Range 语义）──

func parseRangeSimple(rangeHdr string, total int, start, end *int) (bool, error) {
	// 只处理 "bytes=start-end"
	s := rangeHdr
	const prefix = "bytes="
	if len(s) <= len(prefix) || s[:len(prefix)] != prefix {
		return false, nil
	}
	rest := s[len(prefix):]
	dash := -1
	for i, c := range rest {
		if c == '-' {
			dash = i
			break
		}
	}
	if dash < 0 {
		return false, nil
	}
	*start = atoi(rest[:dash])
	endStr := rest[dash+1:]
	if endStr == "" {
		*end = total - 1
	} else {
		*end = atoi(endStr)
	}
	if *end >= total {
		*end = total - 1
	}
	return true, nil
}

func formatContentRange(start, end, total int) string {
	return "bytes " + itoa(int64(start)) + "-" + itoa(int64(end)) + "/" + itoa(int64(total))
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
