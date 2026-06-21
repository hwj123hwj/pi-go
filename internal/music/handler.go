package music

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/earendil-works/pi-go/internal/music/netease"
)

// Handler provides HTTP endpoints for music audio proxy and lyrics.
type Handler struct {
	client *netease.Client
	cache  *Cache
}

// NewHandler creates a new music HTTP handler.
func NewHandler(client *netease.Client, cache *Cache) *Handler {
	return &Handler{client: client, cache: cache}
}

// RegisterRoutes registers music HTTP routes on the given ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /music/audio/{song_id}", h.handleAudio)
	mux.HandleFunc("GET /music/lyrics/{song_id}", h.handleLyrics)
}

// handleAudio proxies the audio stream from NetEase for the given song.
// Strategy: outer URL first, enhance API as fallback.
//
// 支持 Range 请求（seek/拖动进度条）：转发客户端的 Range 头到上游，
// 透传上游的 Content-Range / Accept-Ranges / Content-Length 和状态码（200 或 206）。
// 音频流不能用 http.ServeContent（它要求 ReadSeeker，而上游是流不可 seek），
// 因此采用"透传 Range"方式：上游 CDN 本身支持 Range，我们只做透明代理。
//
// 超时控制：音频流不设整体超时（大文件会被误杀），改用 r.Context() 传递取消——
// 客户端断开时 context 取消，上游请求随之终止，不会挂住。
func (h *Handler) handleAudio(w http.ResponseWriter, r *http.Request) {
	songID, err := parseSongID(r.PathValue("song_id"))
	if err != nil {
		http.Error(w, "invalid song_id", http.StatusBadRequest)
		return
	}

	// Get audio URL (cached)
	audioURL, err := h.getAudioURL(songID)
	if err != nil {
		slog.Error("failed to get audio URL", "song_id", songID, "error", err)
		http.Error(w, fmt.Sprintf("audio not available: %v", err), http.StatusNotFound)
		return
	}

	// Proxy the audio stream, forwarding the client's Range header.
	req, err := http.NewRequestWithContext(r.Context(), "GET", audioURL, nil)
	if err != nil {
		http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Referer", "https://music.163.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	// 转发客户端的 Range 头，让上游返回字节范围（seek 支持）
	if rangeHdr := r.Header.Get("Range"); rangeHdr != "" {
		req.Header.Set("Range", rangeHdr)
	}

	resp, err := audioProxyClient.Do(req)
	if err != nil {
		http.Error(w, "failed to fetch audio", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 透传上游状态码（200 全量 / 206 部分）和 Range 相关头
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		w.Header().Set("Content-Range", cr)
	}
	if ar := resp.Header.Get("Accept-Ranges"); ar != "" {
		w.Header().Set("Accept-Ranges", ar)
	} else {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	// 仅缓存成功响应（200/206）；上游 4xx/5xx（403 防盗链失效、404 下架等）
	// 不可缓存，否则浏览器/CDN 会把错误结果缓存 24h，导致歌曲"永久"播不了。
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}

	// 透传状态码：206 Partial Content 用于 Range 请求，200 用于全量
	w.WriteHeader(resp.StatusCode)

	// Stream the audio
	_, _ = io.Copy(w, resp.Body)
}

// audioProxyClient 代理音频流的 HTTP 客户端。
// 不设 Timeout（整体超时会误杀大文件流），靠请求级 r.Context() 控制生命周期。
// 响应头超时单独设 10s，防止上游挂起时建连卡死。
var audioProxyClient = &http.Client{
	Timeout:       0, // 无整体超时，靠 context 控制
	CheckRedirect: nil,
	Transport: &http.Transport{
		ResponseHeaderTimeout: 10 * time.Second,
	},
}

// handleLyrics returns LRC lyrics as JSON.
func (h *Handler) handleLyrics(w http.ResponseWriter, r *http.Request) {
	songID, err := parseSongID(r.PathValue("song_id"))
	if err != nil {
		http.Error(w, "invalid song_id", http.StatusBadRequest)
		return
	}

	lyrics, err := h.getLyrics(songID)
	if err != nil {
		http.Error(w, fmt.Sprintf("lyrics not available: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	fmt.Fprintf(w, `{"lrc":%q,"tlyric":%q}`, lyrics.LRC, lyrics.TransLRC)
}

func (h *Handler) getAudioURL(songID int64) (string, error) {
	if v := h.cache.Get(AudioKey(songID)); v != nil {
		return v.(string), nil
	}
	url, err := h.client.GetAudioURL(songID)
	if err != nil {
		return "", err
	}
	h.cache.Set(AudioKey(songID), url, TTLAudio)
	return url, nil
}

func (h *Handler) getLyrics(songID int64) (*netease.Lyrics, error) {
	if v := h.cache.Get(LyricsKey(songID)); v != nil {
		return v.(*netease.Lyrics), nil
	}
	lyrics, err := h.client.GetLyrics(songID)
	if err != nil {
		return nil, err
	}
	h.cache.Set(LyricsKey(songID), lyrics, TTLLyrics)
	return lyrics, nil
}

func parseSongID(s string) (int64, error) {
	// Handle path suffixes like ".mp3" from URL patterns
	s = strings.TrimSuffix(s, ".mp3")
	return strconv.ParseInt(s, 10, 64)
}
