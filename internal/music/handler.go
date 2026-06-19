package music

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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

	// Proxy the audio stream
	req, err := http.NewRequestWithContext(r.Context(), "GET", audioURL, nil)
	if err != nil {
		http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Referer", "https://music.163.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "failed to fetch audio", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("upstream returned %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	// Forward headers
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	// Stream the audio
	_, _ = io.Copy(w, resp.Body)
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
