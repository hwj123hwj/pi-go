//go:build integration

package bilibili

import (
	"fmt"
	"testing"
)

func TestSearchLive(t *testing.T) {
	c := NewClient()
	// Test both before and after filtering
	results, err := c.Search("周杰伦", 5)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	fmt.Printf("Results after filter: %d\n", len(results))
	for i, r := range results {
		fmt.Printf("  #%d: %s - %s (%s)\n", i+1, r.Title, r.Author, r.Bvid)
	}
	if len(results) == 0 {
		t.Fatal("expected search results, got 0")
	}
}
