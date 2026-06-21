package music

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
//  HTTP Handler for music audio proxy and lyrics.
//  Supports multi-source routing via composite song IDs.
// ────────────────────────────────────────────────────────────────────────────

// Handler provides HTTP endpoints for music audio proxy and lyrics.
type Handler struct {
	router *SourceRouter
	cache  *Cache
}

// NewHandler creates a new music HTTP handler with multi-source support.
func NewHandler(router *SourceRouter, cache *Cache) *Handler {
	return &Handler{router: router, cache: cache}
}

// RegisterRoutes registers music HTTP routes on the given ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /music/audio/{song_id}", h.handleAudio)
	mux.HandleFunc("GET /music/lyrics/{song_id}", h.handleLyrics)
}

// handleAudio proxies the audio stream for the given song from any source.
func (h *Handler) handleAudio(w http.ResponseWriter, r *http.Request) {
	compositeID := r.PathValue("song_id")
	if compositeID == "" {
		http.Error(w, "missing song_id", http.StatusBadRequest)
		return
	}

	// Parse composite ID to determine source
	src, rawID := parseCompositeID(compositeID)

	// Get audio URL (cached)
	audioURL, err := h.getAudioURL(src, rawID)
	if err != nil {
		slog.Error("failed to get audio URL", "song_id", compositeID, "error", err)
		http.Error(w, fmt.Sprintf("audio not available: %v", err), http.StatusNotFound)
		return
	}

	// Determine referer based on source
	referer := "https://music.163.com/"
	if src == SourceBilibili {
		referer = "https://www.bilibili.com"
	}

	// Proxy the audio stream
	req, err := http.NewRequestWithContext(r.Context(), "GET", audioURL, nil)
	if err != nil {
		http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	if rangeHdr := r.Header.Get("Range"); rangeHdr != "" {
		req.Header.Set("Range", rangeHdr)
	}

	resp, err := audioProxyClient.Do(req)
	if err != nil {
		http.Error(w, "failed to fetch audio", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Passthrough headers
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
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// handleLyrics returns LRC lyrics as JSON.
func (h *Handler) handleLyrics(w http.ResponseWriter, r *http.Request) {
	compositeID := r.PathValue("song_id")
	if compositeID == "" {
		http.Error(w, "missing song_id", http.StatusBadRequest)
		return
	}

	src, rawID := parseCompositeID(compositeID)

	lyrics, err := h.getLyrics(src, rawID)
	if err != nil {
		http.Error(w, fmt.Sprintf("lyrics not available: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	// Escape newlines for JSON string safety
	lrc := strings.ReplaceAll(lyrics.LRC, "\n", "\\n")
	tlyric := strings.ReplaceAll(lyrics.TransLRC, "\n", "\\n")
	fmt.Fprintf(w, `{"lrc":"%s","tlyric":"%s"}`, lrc, tlyric)
}

func (h *Handler) getAudioURL(src Source, rawID string) (string, error) {
	key := AudioKey(string(src), rawID)
	if v := h.cache.Get(key); v != nil {
		return v.(string), nil
	}

	source, err := h.router.Resolve(src)
	if err != nil {
		return "", err
	}

	url, err := source.GetAudioURL(r_context(), rawID)
	if err != nil {
		return "", err
	}

	h.cache.Set(key, url, TTLAudio)
	return url, nil
}

func (h *Handler) getLyrics(src Source, rawID string) (*Lyrics, error) {
	key := LyricsKey(string(src), rawID)
	if v := h.cache.Get(key); v != nil {
		return v.(*Lyrics), nil
	}

	source, err := h.router.Resolve(src)
	if err != nil {
		return nil, err
	}

	lyrics, err := source.GetLyrics(r_context(), rawID)
	if err != nil {
		return nil, err
	}

	h.cache.Set(key, lyrics, TTLLyrics)
	return lyrics, nil
}

// r_context returns a background context. Used for handler-level calls
// where the request context is not available (the actual HTTP request
// context is used in the proxy layer above).
func r_context() context.Context {
	return context.Background()
}

// audioProxyClient is the HTTP client for proxying audio streams.
var audioProxyClient = &http.Client{
	Timeout: 0,
	Transport: &http.Transport{
		ResponseHeaderTimeout: 10 * time.Second,
	},
}

// parseCompositeID parses a composite song ID from a URL path.
// Supports both ":" separator ("netease:12345") and "_" separator
// ("bilibili_BV1xx") used in URL paths.
func parseCompositeID(id string) (Source, string) {
	if idx := strings.Index(id, "_"); idx >= 0 {
		return Source(id[:idx]), id[idx+1:]
	}
	return ParseSourceID(id)
}
