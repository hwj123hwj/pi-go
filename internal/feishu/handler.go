package feishu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Handler processes Feishu messages by calling the pi-agent HTTP API.
type Handler struct {
	piAgentURL string            // e.g. "http://127.0.0.1:8080"
	client     *Client           // Feishu REST client
	sessions   map[string]string // chatKey → sessionID (pi-agent assigned)
	mu         sync.Mutex
	httpClient *http.Client
	workspace  string // workspace root for file path detection
}

// NewHandler creates a new message handler.
func NewHandler(piAgentURL string, client *Client, workspace string) *Handler {
	return &Handler{
		piAgentURL: piAgentURL,
		client:     client,
		sessions:   make(map[string]string),
		httpClient: &http.Client{Timeout: 10 * time.Minute},
		workspace:  workspace,
	}
}

// Handle processes a single Feishu message.
func (h *Handler) Handle(ctx context.Context, msg Message) {
	chatKey := msg.ChatKey()
	messageID := msg.MessageID
	text := strings.TrimSpace(msg.Text)

	slog.Info("handling feishu message", "chatKey", chatKey, "text", text, "messageID", messageID)

	if text == "" {
		return
	}

	// Add "thinking" reaction
	reactionID, _ := h.client.AddReaction(ctx, messageID, "THINKING")
	defer func() {
		_ = h.client.RemoveReaction(ctx, messageID, reactionID)
	}()

	// Slash command handling
	if strings.HasPrefix(text, "/") {
		reply := h.handleSlashCommand(ctx, chatKey, text)
		if reply != "" {
			_, _ = h.client.ReplyMessage(ctx, messageID, chatKey, reply)
		}
		return
	}

	// Regular message: call pi-agent
	h.handleAgentMessage(ctx, chatKey, messageID, text)
}

// handleSlashCommand processes slash commands locally.
func (h *Handler) handleSlashCommand(ctx context.Context, chatKey, text string) string {
	cmd := strings.Fields(text)[0]
	switch cmd {
	case "/new":
		return h.cmdNew(ctx, chatKey)
	case "/compact", "/compress":
		return h.cmdCompact(ctx, chatKey)
	case "/status":
		return h.cmdStatus(chatKey)
	case "/help":
		return h.cmdHelp()
	default:
		return "❓ 未知命令，输入 /help 查看可用命令"
	}
}

func (h *Handler) cmdNew(ctx context.Context, chatKey string) string {
	sessionID, err := h.createSession(ctx)
	if err != nil {
		return fmt.Sprintf("❌ 创建新会话失败: %v", err)
	}
	h.setSession(chatKey, sessionID)
	return "✅ 已开启新对话"
}

func (h *Handler) cmdCompact(ctx context.Context, chatKey string) string {
	sessionID := h.getSession(chatKey)
	if sessionID == "" {
		return "⚠️ 当前没有活跃会话，请先发送一条消息"
	}

	url := fmt.Sprintf("%s/sessions/%s/compact", h.piAgentURL, sessionID)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("❌ 压缩失败: %v", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("❌ 压缩失败 (HTTP %d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		Summary     string `json:"summary"`
		TrimmedFrom int    `json:"trimmed_from"`
		TrimmedTo   int    `json:"trimmed_to"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Sprintf("✅ 压缩完成（响应解析失败）")
	}

	return fmt.Sprintf("✅ 对话已压缩\n📦 压缩前消息数: %d\n📦 压缩后消息数: %d", result.TrimmedFrom, result.TrimmedTo)
}

func (h *Handler) cmdStatus(chatKey string) string {
	sessionID := h.getSession(chatKey)
	if sessionID == "" {
		return "📋 状态: 无活跃会话"
	}
	return fmt.Sprintf("📋 状态: 活跃会话\n🆔 Session ID: %s", sessionID)
}

func (h *Handler) cmdHelp() string {
	return `📖 飞书可用命令:

  /new       新建会话（重置对话历史）
  /compact   压缩对话历史（释放上下文窗口）
  /status    显示当前会话状态
  /help      显示此帮助

💡 其他任何消息都会发送给 AI Agent 处理（支持工具调用）`
}

// handleAgentMessage sends a message to pi-agent and streams the response back.
func (h *Handler) handleAgentMessage(ctx context.Context, chatKey, messageID, text string) {
	slog.Info("handleAgentMessage", "chatKey", chatKey, "sessionID", h.getSession(chatKey))

	// Get or create session
	sessionID := h.getSession(chatKey)
	if sessionID == "" {
		var err error
		sessionID, err = h.createSession(ctx)
		if err != nil {
			slog.Error("create session failed", "error", err)
			_, _ = h.client.ReplyMessage(ctx, messageID, chatKey, "❌ 服务暂不可用，请重试")
			return
		}
		h.setSession(chatKey, sessionID)
		slog.Info("created new session", "chatKey", chatKey, "sessionID", sessionID)
	}

	// Send initial "processing" message
	botMsgID, err := h.client.SendMessage(ctx, chatKey, "⏳ 处理中...", "")
	if err != nil {
		slog.Warn("send processing message failed", "error", err)
	}

	// Call pi-agent chat/stream
	fullText, err := h.streamChat(ctx, sessionID, text, botMsgID)
	if err != nil {
		slog.Error("stream chat failed", "error", err)
		errMsg := "❌ 处理出错，请重试"
		if botMsgID != "" {
			_ = h.client.UpdateMessage(ctx, botMsgID, errMsg)
		} else {
			_, _ = h.client.ReplyMessage(ctx, messageID, chatKey, errMsg)
		}
		return
	}

	// Send final formatted reply (post format)
	if fullText != "" {
		_, _ = h.client.SendMarkdown(ctx, chatKey, fullText, messageID)

		// Detect and send files
		h.sendDetectedFiles(ctx, chatKey, fullText)
	}
}

// streamChat calls POST /chat/stream and collects the full response text.
// It also updates botMsgID periodically with partial content.
func (h *Handler) streamChat(ctx context.Context, sessionID, prompt, botMsgID string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"prompt":     prompt,
		"session_id": sessionID,
	})

	url := fmt.Sprintf("%s/chat/stream", h.piAgentURL)
	slog.Debug("SSE request", "url", url, "sessionID", sessionID, "prompt", prompt)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("SSE response", "status", resp.StatusCode, "contentType", resp.Header.Get("Content-Type"))

	// SSE parsing
	var buf strings.Builder
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Update ticker: periodically update the "processing" message
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				if botMsgID != "" && buf.Len() > 0 {
					_ = h.client.UpdateMessage(context.Background(), botMsgID, buf.String())
				}
			case <-done:
				return
			}
		}
	}()
	defer close(done)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentEvent string

	for scanner.Scan() {
		line := scanner.Text()

		slog.Debug("SSE raw", "line", line)

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			slog.Debug("SSE event", "event", currentEvent, "data", data[:min(len(data), 200)])

			switch currentEvent {
			case "text_delta":
				var ev struct {
					TextDelta string `json:"text_delta"`
				}
				if err := json.Unmarshal([]byte(data), &ev); err == nil && ev.TextDelta != "" {
					buf.WriteString(ev.TextDelta)
				}
			case "done":
				slog.Debug("SSE stream done", "textLen", buf.Len())
				return buf.String(), nil
			case "error":
				return buf.String(), fmt.Errorf("agent error: %s", data)
			case "session_id":
				// Server may return a different session ID; update our mapping
				// This is handled by the initial session creation
			}
			currentEvent = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return buf.String(), fmt.Errorf("read SSE: %w", err)
	}

	return buf.String(), nil
}

// filePathRegex matches file paths in LLM replies.
var filePathRegex = regexp.MustCompile(`(?:^|[\s"'` + "`" + `])((?:/[\w\-./]+)|(?:\./[\w\-./]+))\.(png|jpg|jpeg|gif|webp|svg|bmp|pdf|txt|csv|json|zip|py|js|ts|md|go|rs)`)

// imageExtensions marks file types that should be sent as images.
var imageExtensions = map[string]bool{
	"png": true, "jpg": true, "jpeg": true, "gif": true,
	"webp": true, "svg": true, "bmp": true,
}

// sendDetectedFiles scans the reply text for file paths and sends them.
func (h *Handler) sendDetectedFiles(ctx context.Context, chatID, text string) {
	matches := filePathRegex.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		filePath := m[1] + "." + m[2]
		if seen[filePath] {
			continue
		}
		seen[filePath] = true

		// If workspace is set, only match paths under workspace
		if h.workspace != "" && !strings.HasPrefix(filePath, h.workspace) {
			continue
		}

		// Check file exists
		info, err := os.Stat(filePath)
		if err != nil || info.IsDir() {
			continue
		}

		ext := strings.ToLower(m[2])
		fileName := filepath.Base(filePath)

		if imageExtensions[ext] {
			imageKey, err := h.client.UploadImage(ctx, filePath)
			if err != nil {
				slog.Warn("upload image failed", "path", filePath, "error", err)
				continue
			}
			_, _ = h.client.SendImage(ctx, chatID, imageKey)
		} else {
			fileKey, err := h.client.UploadFile(ctx, filePath, fileName)
			if err != nil {
				slog.Warn("upload file failed", "path", filePath, "error", err)
				continue
			}
			_, _ = h.client.SendFile(ctx, chatID, fileKey)
		}

		slog.Info("sent file to feishu", "path", filePath, "chatID", chatID)
	}
}

// Session management helpers

func (h *Handler) getSession(chatKey string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessions[chatKey]
}

func (h *Handler) setSession(chatKey, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[chatKey] = sessionID
}

func (h *Handler) createSession(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/sessions", h.piAgentURL)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode session response: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("empty session ID")
	}
	return result.ID, nil
}
