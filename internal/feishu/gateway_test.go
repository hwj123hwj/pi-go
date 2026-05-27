package feishu

import (
	"fmt"
	"testing"
)

func TestExtractText(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"normal text", `{"text":"hello world"}`, "hello world"},
		{"empty text", `{"text":""}`, ""},
		{"invalid json", "not json", "not json"},
		{"nil content", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var content *string
			if tt.name != "nil content" {
				content = &tt.content
			}
			got := extractText(content)
			if got != tt.want {
				t.Errorf("extractText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDedup(t *testing.T) {
	g := NewGateway("test", "test", nil)

	// First time: not duplicate
	if g.isDuplicate("msg1") {
		t.Error("msg1 should not be duplicate on first check")
	}

	// Second time: duplicate
	if !g.isDuplicate("msg1") {
		t.Error("msg1 should be duplicate on second check")
	}

	// Different message: not duplicate
	if g.isDuplicate("msg2") {
		t.Error("msg2 should not be duplicate on first check")
	}
}

func TestDedupEviction(t *testing.T) {
	g := NewGateway("test", "test", nil)
	g.maxSeen = 10

	// Add 15 messages (exceeds maxSeen of 10)
	for i := 0; i < 15; i++ {
		msgID := fmt.Sprintf("msg_%03d", i)
		g.isDuplicate(msgID)
	}

	// Map should be at most maxSeen entries
	g.mu.Lock()
	count := len(g.seen)
	g.mu.Unlock()

	if count > g.maxSeen {
		t.Errorf("expected at most %d entries, got %d", g.maxSeen, count)
	}
}

func TestMessageChatKey(t *testing.T) {
	tests := []struct {
		name    string
		msg     Message
		wantKey string
	}{
		{
			name: "group chat uses chatID",
			msg: Message{
				ChatType:     "group",
				ChatID:       "oc_123",
				SenderOpenID: "ou_456",
			},
			wantKey: "oc_123",
		},
		{
			name: "direct chat uses senderOpenID",
			msg: Message{
				ChatType:     "p2p",
				ChatID:       "",
				SenderOpenID: "ou_789",
			},
			wantKey: "ou_789",
		},
		{
			name: "fallback to chatID",
			msg: Message{
				ChatType:     "",
				ChatID:       "oc_fallback",
				SenderOpenID: "",
			},
			wantKey: "oc_fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.ChatKey(); got != tt.wantKey {
				t.Errorf("ChatKey() = %q, want %q", got, tt.wantKey)
			}
		})
	}
}
