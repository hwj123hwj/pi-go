//go:build integration

package netease

import (
	"fmt"
	"testing"
)

// These tests hit the real NetEase Music API (music.163.com).
// They are skipped in CI by default. Run locally with:
//
//	go test -tags integration ./internal/music/netease/

func TestGetRankings(t *testing.T) {
	c := NewClient()
	rankings, err := c.GetRankings()
	if err != nil {
		t.Fatalf("GetRankings: %v", err)
	}
	if len(rankings) == 0 {
		t.Fatal("expected non-empty rankings")
	}
	fmt.Printf("Found %d rankings\n", len(rankings))
	for i, r := range rankings {
		if i >= 5 {
			break
		}
		fmt.Printf("  %d. %s (id:%d, tracks:%d, freq:%s)\n", i+1, r.Name, r.ID, r.TrackCount, r.UpdateFrequency)
	}
}

func TestGetTopList(t *testing.T) {
	c := NewClient()
	detail, err := c.GetTopList(RankSoaring)
	if err != nil {
		t.Fatalf("GetTopList: %v", err)
	}
	fmt.Printf("Playlist: %s\n", detail.Playlist.Name)
	fmt.Printf("Songs: %d\n", len(detail.Songs))
	if len(detail.Songs) > 0 {
		s := detail.Songs[0]
		fmt.Printf("  #1: %s - %s (id:%d)\n", s.Name, s.Artist, s.ID)
	}
}

func TestGetNewSongs(t *testing.T) {
	c := NewClient()
	songs, err := c.GetNewSongs(5)
	if err != nil {
		t.Fatalf("GetNewSongs: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("expected non-empty songs")
	}
	fmt.Printf("Found %d new songs\n", len(songs))
	for i, s := range songs {
		fmt.Printf("  %d. %s - %s (id:%d)\n", i+1, s.Name, s.Artist, s.ID)
	}
}

func TestGetPlaylistDetail(t *testing.T) {
	c := NewClient()
	detail, err := c.GetPlaylistDetail(19723756)
	if err != nil {
		t.Fatalf("GetPlaylistDetail: %v", err)
	}
	fmt.Printf("Playlist: %s (id:%d)\n", detail.Playlist.Name, detail.Playlist.ID)
	fmt.Printf("TrackCount: %d, Songs returned: %d\n", detail.Playlist.TrackCount, len(detail.Songs))
	for i, s := range detail.Songs {
		if i >= 5 {
			break
		}
		fmt.Printf("  %d. %s - %s (id:%d)\n", i+1, s.Name, s.Artist, s.ID)
	}
}
