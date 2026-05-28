package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CardKit 2.0 element IDs
const (
	CardKitStreamingElementID = "streaming_content"
	CardKitFooterElementID    = "footer_content"
	CardKitLoadingElementID   = "loading_icon"
	CardKitLoadingImgKey      = "img_v3_02vb_496bec09-4b43-4773-ad6b-0cdd103cd2bg"
	CardKitMaxContentChars    = 8500
)

// FooterMetrics holds metrics displayed in the card footer.
type FooterMetrics struct {
	Status            string
	ElapsedMs         int64
	Model             string
	InputTokens       int
	OutputTokens      int
	CacheReadTokens   int
	CacheHitRate      float64
	ContextPercentage float64
}

// StreamingCardHandle manages a CardKit 2.0 streaming card session.
type StreamingCardHandle struct {
	MessageID string
	CardID    string
	client    *Client
	sequence  int
	mu        sync.Mutex
	lastPush  time.Time
	minInterval time.Duration
}

// HasCard reports whether the card was successfully created and sent.
func (h *StreamingCardHandle) HasCard() bool {
	return h.MessageID != "" && h.CardID != ""
}

// PushContent pushes cumulative content to the streaming card with throttling.
func (h *StreamingCardHandle) PushContent(content string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	if now.Sub(h.lastPush) < h.minInterval {
		return nil // throttled
	}

	h.sequence++
	truncated := truncateCardText(content)
	if err := h.client.streamCardKitElement(context.Background(), h.CardID, CardKitStreamingElementID, truncated, h.sequence); err != nil {
		h.sequence--
		return err
	}
	h.lastPush = now
	return nil
}

// PushFooter updates the footer metrics.
func (h *StreamingCardHandle) PushFooter(metrics FooterMetrics) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sequence++
	footer := RenderFooterMarkdown(metrics)
	if err := h.client.streamCardKitElement(context.Background(), h.CardID, CardKitFooterElementID, footer, h.sequence); err != nil {
		h.sequence--
		return err
	}
	return nil
}

// Finalize closes streaming mode and writes the final card state.
func (h *StreamingCardHandle) Finalize(content string, metrics *FooterMetrics) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Disable streaming mode
	h.sequence++
	_ = h.client.setCardKitStreamingMode(context.Background(), h.CardID, false, h.sequence)

	// Final card update
	h.sequence++
	finalCard := BuildFinalCard(content, metrics)
	return h.client.updateCardKitCard(context.Background(), h.CardID, finalCard, h.sequence)
}

// RenderFooterMarkdown renders footer metrics as a single markdown line.
func RenderFooterMarkdown(metrics FooterMetrics) string {
	var parts []string
	isError := false

	if metrics.Status != "" {
		lower := strings.ToLower(metrics.Status)
		switch {
		case strings.Contains(lower, "error") || strings.Contains(lower, "failed"):
			parts = append(parts, fmt.Sprintf("<font color='red'>%s</font>", metrics.Status))
			isError = true
		case strings.Contains(lower, "processing") || strings.Contains(lower, "thinking"):
			parts = append(parts, fmt.Sprintf("<font color='grey'>%s</font>", metrics.Status))
		default:
			parts = append(parts, fmt.Sprintf("<font color='green'>%s</font>", metrics.Status))
		}
	}

	if metrics.ElapsedMs > 0 {
		parts = append(parts, fmt.Sprintf("耗时 %s", formatElapsed(metrics.ElapsedMs)))
	}

	if metrics.Model != "" {
		parts = append(parts, metrics.Model)
	}

	if metrics.InputTokens > 0 || metrics.OutputTokens > 0 {
		parts = append(parts, fmt.Sprintf("↑%s ↓%s",
			formatNumber(metrics.InputTokens), formatNumber(metrics.OutputTokens)))
	}

	if metrics.CacheReadTokens > 0 {
		cacheText := fmt.Sprintf("缓存读取 %s", formatNumber(metrics.CacheReadTokens))
		if metrics.CacheHitRate > 0 {
			cacheText += fmt.Sprintf(" (%.1f%%)", metrics.CacheHitRate)
		}
		parts = append(parts, cacheText)
	}

	if metrics.ContextPercentage > 0 {
		remaining := 100 - metrics.ContextPercentage
		if remaining < 0 {
			remaining = 0
		}
		parts = append(parts, fmt.Sprintf("上下文剩余 %.0f%%", remaining))
	}

	if len(parts) == 0 {
		return ""
	}
	text := strings.Join(parts, " · ")
	if isError {
		return fmt.Sprintf("<font color='red'>%s</font>", text)
	}
	return text
}

// BuildStreamingCard builds a CardKit 2.0 streaming initial card.
func BuildStreamingCard(initialContent, initialFooter string) map[string]any {
	elements := []any{
		map[string]any{
			"tag":         "markdown",
			"element_id":  CardKitStreamingElementID,
			"content":     truncateCardText(initialContent),
			"text_align":  "left",
			"text_size":   "normal_v2",
		},
		map[string]any{
			"tag":        "markdown",
			"element_id": CardKitLoadingElementID,
			"content":    " ",
			"icon": map[string]any{
				"tag":    "custom_icon",
				"img_key": CardKitLoadingImgKey,
				"size":   "16px 16px",
			},
		},
	}

	footerContent := initialFooter
	if footerContent == "" {
		footerContent = " "
	}
	elements = append(elements, map[string]any{
		"tag":        "markdown",
		"element_id": CardKitFooterElementID,
		"content":    footerContent,
		"text_size":  "notation",
	})

	return map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"streaming_mode": true,
			"summary": map[string]any{
				"content":     "Processing...",
				"i18n_content": map[string]string{"zh_cn": "处理中...", "en_us": "Processing..."},
			},
		},
		"body": map[string]any{
			"elements": elements,
		},
	}
}

// BuildFinalCard builds a CardKit 2.0 final (non-streaming) card.
func BuildFinalCard(content string, metrics *FooterMetrics) map[string]any {
	elements := []any{
		map[string]any{
			"tag":        "markdown",
			"element_id": CardKitStreamingElementID,
			"content":    truncateCardText(content),
			"text_align": "left",
			"text_size":  "normal_v2",
		},
	}

	var footerContent string
	if metrics != nil {
		footerContent = RenderFooterMarkdown(*metrics)
	}
	if footerContent != "" {
		elements = append(elements, map[string]any{
			"tag":        "markdown",
			"element_id": CardKitFooterElementID,
			"content":    footerContent,
			"text_size":  "notation",
		})
	}

	summaryText := strings.ReplaceAll(content, "*", "")
	summaryText = strings.ReplaceAll(summaryText, "_", "")
	summaryText = strings.ReplaceAll(summaryText, "#", "")
	summaryText = strings.ReplaceAll(summaryText, "`", "")
	summaryText = strings.TrimSpace(summaryText)
	if len(summaryText) > 120 {
		summaryText = summaryText[:120]
	}
	if summaryText == "" {
		summaryText = "Done"
	}

	return map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"streaming_mode": false,
			"summary":        map[string]any{"content": summaryText},
		},
		"body": map[string]any{
			"elements": elements,
		},
	}
}

// --- CardKit 底层 API ---

// createCardKitCard creates a CardKit 2.0 card entity.
func (c *Client) createCardKitCard(ctx context.Context, card map[string]any) (string, error) {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return "", err
	}

	cardJSON, _ := json.Marshal(card)
	body := fmt.Sprintf(`{"type":"card_json","data":%s}`, jsonString(string(cardJSON)))

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/cardkit/v1/cards",
		strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			CardID string `json:"card_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode card response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("cardkit.card.create failed (code %d): %s", result.Code, result.Msg)
	}

	slog.Info("cardkit card created", "cardID", result.Data.CardID)
	return result.Data.CardID, nil
}

// sendCardKitMessage sends a CardKit card as an IM message.
func (c *Client) sendCardKitMessage(ctx context.Context, chatID, cardID, replyTo string) (string, error) {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return "", err
	}

	content := fmt.Sprintf(`{"type":"card","data":{"card_id":"%s"}}`, cardID)

	var url string
	var body string
	if replyTo != "" {
		url = fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages/%s/reply", replyTo)
		body = fmt.Sprintf(`{"msg_type":"interactive","content":%s}`, jsonString(content))
	} else {
		url = "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id"
		body = fmt.Sprintf(`{"receive_id":"%s","msg_type":"interactive","content":%s}`, chatID, jsonString(content))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode message response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("send cardkit message failed (code %d): %s", result.Code, result.Msg)
	}

	return result.Data.MessageID, nil
}

// streamCardKitElement pushes content to a specific card element.
func (c *Client) streamCardKitElement(ctx context.Context, cardID, elementID, content string, sequence int) error {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/cardkit/v1/cards/%s/elements/%s/content",
		cardID, elementID)
	body := fmt.Sprintf(`{"content":%s,"sequence":%d}`, jsonString(content), sequence)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stream element failed (HTTP %d): %s", resp.StatusCode, string(data[:min(len(data), 200)]))
	}
	return nil
}

// setCardKitStreamingMode enables or disables streaming mode on a card.
func (c *Client) setCardKitStreamingMode(ctx context.Context, cardID string, enabled bool, sequence int) error {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/cardkit/v1/cards/%s/settings", cardID)
	body := fmt.Sprintf(`{"settings":{"streaming_mode":%v},"sequence":%d}`, enabled, sequence)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set streaming mode failed (HTTP %d): %s", resp.StatusCode, string(data[:min(len(data), 200)]))
	}
	return nil
}

// updateCardKitCard fully updates a card (used for final state).
func (c *Client) updateCardKitCard(ctx context.Context, cardID string, card map[string]any, sequence int) error {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return err
	}

	cardJSON, _ := json.Marshal(card)
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/cardkit/v1/cards/%s", cardID)
	body := fmt.Sprintf(`{"card":{"type":"card_json","data":%s},"sequence":%d}`, jsonString(string(cardJSON)), sequence)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update card failed (HTTP %d): %s", resp.StatusCode, string(data[:min(len(data), 200)]))
	}
	return nil
}

// SendStreamingCard creates a CardKit streaming card and returns a handle.
// Returns a handle with nil MessageID if CardKit creation fails (caller should fall back).
func (c *Client) SendStreamingCard(ctx context.Context, chatID, initialContent, replyTo string) *StreamingCardHandle {
	card := BuildStreamingCard(initialContent, "")
	cardID, err := c.createCardKitCard(ctx, card)
	if err != nil {
		slog.Warn("cardkit create failed, will fall back to PATCH text", "error", err)
		return &StreamingCardHandle{client: c}
	}

	messageID, err := c.sendCardKitMessage(ctx, chatID, cardID, replyTo)
	if err != nil {
		slog.Warn("cardkit send message failed", "error", err)
		return &StreamingCardHandle{client: c, CardID: cardID}
	}

	return &StreamingCardHandle{
		MessageID:   messageID,
		CardID:      cardID,
		client:      c,
		sequence:    1,
		lastPush:    time.Now(),
		minInterval: 1500 * time.Millisecond,
	}
}

// truncateCardText truncates content to CardKitMaxContentChars.
func truncateCardText(text string) string {
	if len(text) <= CardKitMaxContentChars {
		return text
	}
	return text[:CardKitMaxContentChars] + "\n\n（内容过长，已截断）"
}

// formatElapsed formats milliseconds as human-readable duration.
func formatElapsed(ms int64) string {
	seconds := float64(ms) / 1000
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	}
	minutes := int(seconds) / 60
	remaining := int(seconds) % 60
	return fmt.Sprintf("%dm %ds", minutes, remaining)
}

// formatNumber formats an integer with locale-style thousand separators.
func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// jsonString wraps a string as a JSON string value.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
