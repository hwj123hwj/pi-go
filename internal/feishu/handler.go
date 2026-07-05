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

// ChatRoute binds a Feishu chat to a local project and pi-agent session.
type ChatRoute struct {
	SessionID   string `json:"session_id"`
	ProjectRoot string `json:"project_root,omitempty"`
	ChatName    string `json:"chat_name,omitempty"`
}

// Handler processes Feishu messages by calling the pi-agent HTTP API.
type Handler struct {
	piAgentURL string // e.g. "http://127.0.0.1:8080"
	appID      string // Feishu app ID for permission links
	client     *Client
	gateway    *Gateway

	routes     map[string]*ChatRoute // chatKey → route (session + project)
	routesFile string                // persistent route config file path
	routesMu   sync.Mutex

	httpClient *http.Client
	workspace  string // default workspace root for file path detection

	// Sender context: per-chat sender OpenID for tool callbacks.
	// Keyed by chatKey to avoid multi-chat race conditions.
	senders   map[string]string
	senderMu  sync.Mutex
}

// NewHandler creates a new message handler.
func NewHandler(piAgentURL, appID string, client *Client, workspace string) *Handler {
	h := &Handler{
		piAgentURL: piAgentURL,
		appID:      appID,
		client:     client,
		routes:     make(map[string]*ChatRoute),
		routesFile: defaultRoutesFile(),
		httpClient: &http.Client{Timeout: 10 * time.Minute},
		workspace:  workspace,
		senders:    make(map[string]string),
	}
	h.loadRoutes()
	return h
}

func defaultRoutesFile() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".pi-go")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "feishu-routes.json")
}

func (h *Handler) loadRoutes() {
	data, err := os.ReadFile(h.routesFile)
	if err != nil {
		return // file may not exist yet
	}
	var routes map[string]*ChatRoute
	if err := json.Unmarshal(data, &routes); err != nil {
		slog.Warn("failed to parse routes file", "error", err)
		return
	}
	h.routes = routes
	slog.Info("loaded feishu routes", "count", len(routes))
}

func (h *Handler) saveRoutes() {
	data, err := json.MarshalIndent(h.routes, "", "  ")
	if err != nil {
		slog.Warn("failed to marshal routes", "error", err)
		return
	}
	if err := os.WriteFile(h.routesFile, data, 0o644); err != nil {
		slog.Warn("failed to save routes file", "error", err)
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

	// Store sender context for tool callbacks
	if msg.SenderOpenID != "" {
		h.storeSender(chatKey, msg.SenderOpenID)
	}

	// Add "thinking" reaction
	reactionID, _ := h.client.AddReaction(ctx, messageID, "THINKING")
	defer func() {
		_ = h.client.RemoveReaction(ctx, messageID, reactionID)
	}()

	// Slash command handling
	if strings.HasPrefix(text, "/") {
		reply := h.handleSlashCommand(ctx, chatKey, msg.SenderOpenID, msg.ChatType, text)
		if reply != "" {
			_, _ = h.client.ReplyMessage(ctx, messageID, chatKey, reply)
		}
		return
	}

	// Regular message: call pi-agent
	h.handleAgentMessage(ctx, chatKey, messageID, text)
}

// handleSlashCommand processes slash commands.
// Known commands are handled locally; unknown ones are forwarded to pi-agent.
func (h *Handler) handleSlashCommand(ctx context.Context, chatKey, senderOpenID, chatType, text string) string {
	cmd := strings.Fields(text)[0]
	switch cmd {
	case "/new":
		return h.cmdNew(ctx, chatKey)
	case "/compact", "/compress":
		return h.cmdCompact(ctx, chatKey)
	case "/status":
		return h.cmdStatus(chatKey)
	case "/project":
		return h.cmdProject(ctx, chatKey, senderOpenID, chatType, text)
	case "/help":
		return h.cmdHelp()
	default:
		return h.forwardCommand(ctx, chatKey, text)
	}
}

func (h *Handler) cmdNew(ctx context.Context, chatKey string) string {
	sessionID, err := h.createSession(ctx)
	if err != nil {
		return fmt.Sprintf("❌ 创建新会话失败: %v", err)
	}
	route := h.getRoute(chatKey)
	if route == nil {
		route = &ChatRoute{}
	}
	route.SessionID = sessionID
	h.setRoute(chatKey, route)
	return "✅ 已开启新对话"
}

func (h *Handler) cmdCompact(ctx context.Context, chatKey string) string {
	route := h.getRoute(chatKey)
	if route == nil || route.SessionID == "" {
		return "⚠️ 当前没有活跃会话，请先发送一条消息"
	}

	url := fmt.Sprintf("%s/sessions/%s/compact", h.piAgentURL, route.SessionID)
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
	route := h.getRoute(chatKey)
	if route == nil || route.SessionID == "" {
		return "📋 状态: 无活跃会话"
	}
	info := fmt.Sprintf("📋 状态: 活跃会话\n🆔 Session ID: %s", route.SessionID)
	if route.ProjectRoot != "" {
		info += fmt.Sprintf("\n📁 项目目录: %s", route.ProjectRoot)
	}
	return info
}

func (h *Handler) cmdHelp() string {
	return `📖 飞书可用命令:

  /new       新建会话（重置对话历史）
  /compact   压缩对话历史（释放上下文窗口）
  /status    显示当前会话状态
  /project   项目管理（创建群 + 绑定项目目录）
  /help      显示此帮助

💡 其他任何消息都会发送给 AI Agent 处理（支持工具调用）`
}

func (h *Handler) cmdProject(ctx context.Context, chatKey, senderOpenID, chatType, text string) string {
	args := strings.Fields(text)
	if len(args) < 2 {
		return `📁 项目管理命令:
  /project create <项目路径> <群名称>  — 创建项目群（仅限私聊）
  /project list                       — 列出所有项目绑定`
	}

	switch args[1] {
	case "create":
		return h.cmdProjectCreate(ctx, chatKey, senderOpenID, chatType, args[2:])
	case "list":
		return h.cmdProjectList()
	default:
		return fmt.Sprintf("❌ 未知子命令: %s\n可用: create, list", args[1])
	}
}

func (h *Handler) cmdProjectCreate(ctx context.Context, chatKey, senderOpenID, chatType string, args []string) string {
	if chatType != "p2p" {
		return "❌ 创建项目群仅限私聊中使用"
	}
	if senderOpenID == "" {
		return "❌ 无法获取用户信息"
	}
	if len(args) < 2 {
		return "❌ 用法: /project create <项目路径> <群名称>"
	}

	projectPath := args[0]
	groupName := strings.Join(args[1:], " ")

	// Validate path exists
	info, err := os.Stat(projectPath)
	if err != nil {
		return fmt.Sprintf("❌ 路径不存在: %v", err)
	}
	if !info.IsDir() {
		return fmt.Sprintf("❌ 路径不是目录: %s", projectPath)
	}

	// Create group chat
	chatID, err := h.client.CreateGroupChat(ctx, groupName, "Pi Agent 项目协作群", []string{senderOpenID})
	if err != nil {
		return fmt.Sprintf("❌ 创建群失败: %v", err)
	}

	// Bind route
	h.setRoute(chatID, &ChatRoute{
		ProjectRoot: projectPath,
		ChatName:    groupName,
	})

	// Send welcome message to the new group
	welcome := fmt.Sprintf("👋 项目群已创建！\n📁 项目目录: `%s`\n\n请在本群中直接发送消息与 AI Agent 对话。", projectPath)
	_, _ = h.client.SendMessage(ctx, chatID, welcome, "")

	return fmt.Sprintf("✅ 项目群创建成功！\n📌 群名: %s\n📁 项目: %s\n🆔 Chat ID: %s\n\n已在群中发送欢迎消息，请切换到新群开始使用。", groupName, projectPath, chatID)
}

func (h *Handler) cmdProjectList() string {
	h.routesMu.Lock()
	defer h.routesMu.Unlock()

	if len(h.routes) == 0 {
		return "📋 暂无项目绑定"
	}

	var sb strings.Builder
	sb.WriteString("📋 项目绑定列表:\n\n")
	for chatKey, route := range h.routes {
		name := route.ChatName
		if name == "" {
			name = chatKey
		}
		sb.WriteString(fmt.Sprintf("  📌 %s\n     📁 %s\n     🆔 %s\n\n", name, route.ProjectRoot, chatKey))
	}
	return sb.String()
}

// handleAgentMessage sends a message to pi-agent and streams the response back.
// Prefers CardKit streaming card; falls back to plain text PATCH if card creation fails.
func (h *Handler) handleAgentMessage(ctx context.Context, chatKey, messageID, text string) {
	// Get or create session
	route := h.getRoute(chatKey)
	if route == nil || route.SessionID == "" {
		sessionID, err := h.createSession(ctx)
		if err != nil {
			slog.Error("create session failed", "error", err)
			_, _ = h.client.ReplyMessage(ctx, messageID, chatKey, "❌ 服务暂不可用，请重试")
			return
		}
		if route == nil {
			route = &ChatRoute{}
		}
		route.SessionID = sessionID
		h.setRoute(chatKey, route)
		slog.Info("created new session", "chatKey", chatKey, "sessionID", sessionID)
	}

	slog.Info("handleAgentMessage", "chatKey", chatKey, "sessionID", route.SessionID, "projectRoot", route.ProjectRoot)

	// Try CardKit streaming card
	card := h.client.SendStreamingCard(ctx, chatKey, "⏳ 正在思考...", messageID)

	if card.HasCard() {
		h.handleWithCard(ctx, chatKey, messageID, route.SessionID, text, card)
	} else {
		h.handleWithTextFallback(ctx, chatKey, messageID, route.SessionID, text)
	}
}

// handleWithCard streams the agent response into a CardKit streaming card.
func (h *Handler) handleWithCard(ctx context.Context, chatKey, messageID, sessionID, text string, card *StreamingCardHandle) {
	startTime := time.Now()

	fullText, err := h.streamChat(ctx, sessionID, text, card)
	elapsed := time.Since(startTime)

	if err != nil {
		slog.Error("stream chat failed", "error", err)
		_ = card.Finalize("❌ 处理出错，请重试", &FooterMetrics{
			Status:    "Error",
			ElapsedMs: elapsed.Milliseconds(),
		})
		return
	}

	// Finalize card with result
	metrics := &FooterMetrics{
		Status:    "已完成",
		ElapsedMs: elapsed.Milliseconds(),
	}
	if err := card.Finalize(fullText, metrics); err != nil {
		slog.Warn("card finalize failed", "error", err)
	}

	if fullText != "" {
		h.sendDetectedFiles(ctx, chatKey, fullText, h.workspaceFor(chatKey))
	}
}

// handleWithTextFallback uses plain text messages with periodic PATCH updates.
func (h *Handler) handleWithTextFallback(ctx context.Context, chatKey, messageID, sessionID, text string) {
	botMsgID, err := h.client.SendMessage(ctx, chatKey, "⏳ 处理中...", "")
	if err != nil {
		slog.Warn("send processing message failed", "error", err)
	}

	fullText, err := h.streamChat(ctx, sessionID, text, nil)
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

	if fullText != "" {
		_, _ = h.client.SendMarkdown(ctx, chatKey, fullText, messageID)
		h.sendDetectedFiles(ctx, chatKey, fullText, h.workspaceFor(chatKey))
	}
}

// streamChat calls POST /chat/stream and collects the full response text.
// When card is non-nil, pushes content to the streaming card; otherwise uses periodic UpdateMessage.
func (h *Handler) streamChat(ctx context.Context, sessionID, prompt string, card *StreamingCardHandle) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"prompt":     prompt,
		"session_id": sessionID,
	})

	url := fmt.Sprintf("%s/chat/stream", h.piAgentURL)
	slog.Debug("SSE request", "url", url, "sessionID", sessionID)
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

	var buf strings.Builder
	startTime := time.Now()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentEvent string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			switch currentEvent {
			case "text_delta":
				var ev struct {
					TextDelta string `json:"text_delta"`
				}
				if err := json.Unmarshal([]byte(data), &ev); err == nil && ev.TextDelta != "" {
					buf.WriteString(ev.TextDelta)
					h.pushContentToCard(card, buf.String(), startTime)
				}

			case "tool_start":
				var ev struct {
					ToolName string `json:"tool_name"`
				}
				if err := json.Unmarshal([]byte(data), &ev); err == nil && ev.ToolName != "" {
					slog.Debug("tool started", "tool", ev.ToolName)
					if card != nil {
						card.PushFooter(FooterMetrics{
							Status:    fmt.Sprintf("🔧 %s", ev.ToolName),
							ElapsedMs: time.Since(startTime).Milliseconds(),
						})
					}
				}

			case "done":
				slog.Debug("SSE stream done", "textLen", buf.Len())
				return buf.String(), nil

			case "error":
				return buf.String(), fmt.Errorf("agent error: %s", data)
			}
			currentEvent = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return buf.String(), fmt.Errorf("read SSE: %w", err)
	}

	return buf.String(), nil
}

// pushContentToCard pushes content to the streaming card (no-op if card is nil).
func (h *Handler) pushContentToCard(card *StreamingCardHandle, content string, startTime time.Time) {
	if card == nil {
		return
	}
	card.PushContent(content)
	card.PushFooter(FooterMetrics{
		Status:    "思考中...",
		ElapsedMs: time.Since(startTime).Milliseconds(),
	})
}

// filePathRegex matches file paths in LLM replies.
var filePathRegex = regexp.MustCompile(`(?:^|[\s"'` + "`" + `])((?:/[\w\-./]+)|(?:\./[\w\-./]+))\.(png|jpg|jpeg|gif|webp|svg|bmp|pdf|txt|csv|json|zip|py|js|ts|md|go|rs)`)

// imageExtensions marks file types that should be sent as images.
var imageExtensions = map[string]bool{
	"png": true, "jpg": true, "jpeg": true, "gif": true,
	"webp": true, "svg": true, "bmp": true,
}

// sendDetectedFiles scans the reply text for file paths and sends them.
func (h *Handler) sendDetectedFiles(ctx context.Context, chatID, text, workspace string) {
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
		if workspace != "" && !strings.HasPrefix(filePath, workspace) {
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

// SetGateway sets the gateway reference (called after gateway creation in bridge startup).
func (h *Handler) SetGateway(gw *Gateway) {
	h.gateway = gw
}

// storeSender stores the sender OpenID for a specific chat (for tool callbacks).
// Keyed by chatKey to avoid multi-chat race conditions.
func (h *Handler) storeSender(chatKey string, senderOpenID string) {
	h.senderMu.Lock()
	defer h.senderMu.Unlock()
	h.senders[chatKey] = senderOpenID
}

// getSender retrieves the sender OpenID for a specific chat.
func (h *Handler) getSender(_ context.Context, chatKey string) string {
	h.senderMu.Lock()
	defer h.senderMu.Unlock()
	return h.senders[chatKey]
}

// getAnySender returns any known sender (best-effort fallback for tool callbacks
// where we can't determine the chat context).
func (h *Handler) getAnySender() string {
	h.senderMu.Lock()
	defer h.senderMu.Unlock()
	for _, id := range h.senders {
		if id != "" {
			return id
		}
	}
	return ""
}

func (h *Handler) getRoute(chatKey string) *ChatRoute {
	h.routesMu.Lock()
	defer h.routesMu.Unlock()
	return h.routes[chatKey]
}

func (h *Handler) setRoute(chatKey string, route *ChatRoute) {
	h.routesMu.Lock()
	defer h.routesMu.Unlock()
	h.routes[chatKey] = route
	h.saveRoutes()
}

// workspaceFor returns the project root for a chat, falling back to the default workspace.
func (h *Handler) workspaceFor(chatKey string) string {
	if r := h.getRoute(chatKey); r != nil && r.ProjectRoot != "" {
		return r.ProjectRoot
	}
	return h.workspace
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

// forwardCommand sends an unrecognized slash command to pi-agent for execution.
// This enables all 14 built-in commands (model, goal, tools, etc.) without
// the bridge needing to know about each one.
func (h *Handler) forwardCommand(ctx context.Context, chatKey, text string) string {
	route := h.getRoute(chatKey)
	if route == nil || route.SessionID == "" {
		return "⚠️ 当前没有活跃会话，请先发送一条消息"
	}

	body, _ := json.Marshal(map[string]string{"command": text})
	url := fmt.Sprintf("%s/sessions/%s/command", h.piAgentURL, route.SessionID)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("❌ 命令执行失败: %v", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.Unmarshal(data, &errResp)
		if errResp.Error != "" {
			return fmt.Sprintf("❌ %s", errResp.Error)
		}
		return fmt.Sprintf("❌ 命令执行失败 (HTTP %d)", resp.StatusCode)
	}

	var result struct {
		Output      string `json:"output"`
		ShouldQuery bool   `json:"should_query"`
		QueryPrompt string `json:"query_prompt"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Sprintf("❌ 响应解析失败: %v", err)
	}

	// If the command wants to trigger an agent query (e.g. /goal), forward it
	if result.ShouldQuery && result.QueryPrompt != "" {
		go h.handleAgentMessage(context.Background(), chatKey, "", result.QueryPrompt)
		if result.Output != "" {
			return result.Output
		}
		return ""
	}

	return result.Output
}
