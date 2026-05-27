package feishu

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"

	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// Message represents a parsed Feishu message event.
type Message struct {
	Text         string
	MessageID    string
	ChatID       string
	ChatType     string // "p2p" or "group"
	SenderOpenID string
	MsgType      string
}

// MessageHandler is called for each incoming message.
type MessageHandler func(ctx context.Context, msg Message)

// Gateway manages a WebSocket long connection to Feishu event subscription.
type Gateway struct {
	appID     string
	appSecret string
	handler   MessageHandler

	// Dedup state
	seen    map[string]struct{}
	mu      sync.Mutex
	maxSeen int

	// Text choice callback (reserved for future ask_user_question support)
	textChoiceCb   func(msg Message) bool
	textChoiceCbMu sync.RWMutex
}

// NewGateway creates a new WebSocket gateway.
func NewGateway(appID, appSecret string, handler MessageHandler) *Gateway {
	return &Gateway{
		appID:     appID,
		appSecret: appSecret,
		handler:   handler,
		seen:      make(map[string]struct{}),
		maxSeen:   1000,
	}
}

// SetTextChoiceCallback sets a callback for text choice interception.
// Reserved for future use when agent supports ask_user_question tool.
func (g *Gateway) SetTextChoiceCallback(cb func(msg Message) bool) {
	g.textChoiceCbMu.Lock()
	defer g.textChoiceCbMu.Unlock()
	g.textChoiceCb = cb
}

// Start connects to Feishu WS and blocks until ctx is cancelled.
func (g *Gateway) Start(ctx context.Context) error {
	dispatcher := larkdispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			g.handleEvent(ctx, event)
			return nil
		})

	wsClient := larkws.NewClient(g.appID, g.appSecret,
		larkws.WithEventHandler(dispatcher),
	)

	slog.Info("feishu gateway connecting via WebSocket...")

	err := wsClient.Start(ctx)
	if err != nil {
		slog.Error("feishu ws client stopped", "error", err)
	}
	return err
}

// handleEvent processes a single incoming message event.
func (g *Gateway) handleEvent(ctx context.Context, event *larkim.P2MessageReceiveV1) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return
	}

	msg := event.Event.Message
	sender := event.Event.Sender

	// Filter: only handle text messages
	if msg.MessageType != nil && *msg.MessageType != "text" {
		return
	}

	// Filter: ignore non-user messages (bot's own messages)
	if sender != nil && sender.SenderType != nil && *sender.SenderType != "user" {
		return
	}

	// Extract text content
	text := extractText(msg.Content)

	// Strip @bot mentions
	if msg.Mentions != nil {
		for _, m := range msg.Mentions {
			if m.Key != nil {
				text = strings.ReplaceAll(text, *m.Key, "")
			}
		}
		text = strings.TrimSpace(text)
	}

	if text == "" {
		return
	}

	// Build Message
	messageID := derefStr(msg.MessageId)
	chatID := derefStr(msg.ChatId)
	chatType := derefStr(msg.ChatType)
	senderOpenID := ""
	if sender != nil && sender.SenderId != nil {
		senderOpenID = derefStr(sender.SenderId.OpenId)
	}

	feishuMsg := Message{
		Text:         text,
		MessageID:    messageID,
		ChatID:       chatID,
		ChatType:     chatType,
		SenderOpenID: senderOpenID,
		MsgType:      "text",
	}

	// Dedup by messageID
	if g.isDuplicate(messageID) {
		slog.Debug("skipping duplicate message", "messageID", messageID)
		return
	}

	// Check text choice callback (reserved)
	g.textChoiceCbMu.RLock()
	cb := g.textChoiceCb
	g.textChoiceCbMu.RUnlock()
	if cb != nil && cb(feishuMsg) {
		slog.Debug("message consumed by text choice callback", "messageID", messageID)
		return
	}

	// Dispatch to handler with a fresh context (not the SDK's event context
	// which gets canceled as soon as the callback returns).
	go g.handler(context.Background(), feishuMsg)
}

// isDuplicate checks and records a message ID for dedup.
func (g *Gateway) isDuplicate(messageID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.seen[messageID]; ok {
		return true
	}

	g.seen[messageID] = struct{}{}

	// Evict entries when over limit
	if len(g.seen) > g.maxSeen {
		count := len(g.seen) - 900
		for k := range g.seen {
			delete(g.seen, k)
			count--
			if count <= 0 {
				break
			}
		}
	}

	return false
}

// extractText parses the message content JSON to get the text field.
func extractText(content *string) string {
	if content == nil {
		return ""
	}

	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(*content), &parsed); err != nil {
		return *content
	}
	return parsed.Text
}

// ChatKey returns the routing key for a message:
// group chat → chatID, direct chat → senderOpenID.
func (m *Message) ChatKey() string {
	if m.ChatType == "group" && m.ChatID != "" {
		return m.ChatID
	}
	if m.SenderOpenID != "" {
		return m.SenderOpenID
	}
	return m.ChatID
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
