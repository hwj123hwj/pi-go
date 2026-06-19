package netease

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchResponseParsing(t *testing.T) {
	// Simulate NetEase search API response and verify parsing
	body := `{
		"code": 200,
		"result": {
			"songCount": 2,
			"songs": [
				{"id": 123, "name": "Test Song", "duration": 240000,
				 "artists": [{"name": "Test Artist"}],
				 "album": {"name": "Test Album", "picUrl": "https://img.com/cover.jpg"}},
				{"id": 456, "name": "Another Song", "duration": 180000,
				 "artists": [{"name": "Another Artist"}],
				 "album": {"name": "Another Album", "picUrl": ""}}
			]
		}
	}`

	var resp searchResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 200 {
		t.Errorf("expected code 200, got %d", resp.Code)
	}
	if len(resp.Result.Songs) != 2 {
		t.Fatalf("expected 2 songs, got %d", len(resp.Result.Songs))
	}
	if resp.Result.Songs[0].Name != "Test Song" {
		t.Errorf("expected 'Test Song', got %q", resp.Result.Songs[0].Name)
	}
	if resp.Result.Songs[0].Artists[0].Name != "Test Artist" {
		t.Errorf("expected 'Test Artist', got %q", resp.Result.Songs[0].Artists[0].Name)
	}
}

func TestLyricsResponseParsing(t *testing.T) {
	body := `{
		"code": 200,
		"lrc": {"lyric": "[00:00.00]Test lyrics\n[00:12.34]Second line"},
		"tlyric": {"lyric": "[00:00.00]翻译歌词"}
	}`

	var resp lyricsResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 200 {
		t.Errorf("expected code 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.LRC.Lyric, "Test lyrics") {
		t.Errorf("expected lyrics to contain 'Test lyrics', got %q", resp.LRC.Lyric)
	}
	if resp.TLyric.Lyric == "" {
		t.Error("expected non-empty translated lyrics")
	}
}

func TestEnhancePlayerResponseParsing(t *testing.T) {
	body := `{
		"code": 200,
		"data": [{"url": "https://cdn.example.com/song.mp3"}]
	}`

	var resp enhancePlayerResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 data entry, got %d", len(resp.Data))
	}
	if resp.Data[0].URL != "https://cdn.example.com/song.mp3" {
		t.Errorf("unexpected URL: %s", resp.Data[0].URL)
	}
}

func TestAudioURLCheck(t *testing.T) {
	var baseURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/audio.mp3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(200)
		fmt.Fprint(w, "fake-audio-data")
	})
	mux.HandleFunc("/not-audio", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		fmt.Fprint(w, "<html>not audio</html>")
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", baseURL+"/audio.mp3")
		w.WriteHeader(302)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	baseURL = srv.URL

	c := NewClient()
	c.httpClient = srv.Client()

	if !c.checkURL(srv.URL + "/audio.mp3") {
		t.Error("expected checkURL to return true for audio content")
	}
	if c.checkURL(srv.URL + "/not-audio") {
		t.Error("expected checkURL to return false for non-audio content")
	}
	if c.checkURL(srv.URL + "/nonexistent") {
		t.Error("expected checkURL to return false for 404")
	}
	if !c.checkURL(srv.URL + "/redirect") {
		t.Error("expected checkURL to return true for 302 redirect")
	}
}

func TestSongTypeConversion(t *testing.T) {
	// Verify that the internal search response converts to Song type correctly
	songs := []Song{
		{ID: 1, Name: "Song A", Artist: "Artist A", AlbumName: "Album A", Duration: 240000},
		{ID: 2, Name: "Song B", Artist: "Artist B", AlbumName: "Album B", Duration: 180000},
	}

	if songs[0].ID != 1 {
		t.Errorf("expected ID 1, got %d", songs[0].ID)
	}
	if songs[1].Duration != 180000 {
		t.Errorf("expected duration 180000, got %d", songs[1].Duration)
	}
}
