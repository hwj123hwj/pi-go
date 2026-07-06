package feishu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// ToolCallbackRequest mirrors agent.ToolCallbackRequest for the bridge side.
type ToolCallbackRequest struct {
	ToolName  string          `json:"tool_name"`
	Params    json.RawMessage `json:"params"`
	SessionID string          `json:"session_id,omitempty"`
}

// ToolCallbackResponse mirrors agent.ToolCallbackResponse for the bridge side.
type ToolCallbackResponse struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// RegisterTool registers the create_project_group tool with the pi-agent server.
func RegisterTool(piAgentURL, callbackURL string) error {
	toolDef := map[string]any{
		"name":        "create_project_group",
		"description": "创建飞书项目协作群，将指定本地项目目录绑定到一个新建的飞书群。群内所有对话自动使用该项目目录作为工作空间。",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"project_path": map[string]any{
					"type":        "string",
					"description": "本地项目的绝对路径，例如 /Users/dev/my-project",
				},
				"group_name": map[string]any{
					"type":        "string",
					"description": "飞书群名称，例如 my-project 项目协作",
				},
				"task": map[string]any{
					"type":        "string",
					"description": "可选。写入 worktree 根目录 TASK.md 的任务目标或用户原始需求。",
				},
			},
			"required": []string{"project_path", "group_name"},
		},
		"callback_url": callbackURL,
	}

	body, _ := json.Marshal(toolDef)
	url := fmt.Sprintf("%s/tools/register", piAgentURL)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("register tool: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register tool failed (HTTP %d): %s", resp.StatusCode, string(data))
	}

	slog.Info("registered create_project_group tool", "callback", callbackURL)
	return nil
}

// HandleToolCallback handles the HTTP callback from pi-agent for tool execution.
func (h *Handler) HandleToolCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeToolError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ToolCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeToolError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var params struct {
		ProjectPath string `json:"project_path"`
		GroupName   string `json:"group_name"`
		Task        string `json:"task"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeToolError(w, http.StatusBadRequest, "invalid parameters")
		return
	}
	if params.ProjectPath == "" || params.GroupName == "" {
		writeToolError(w, http.StatusBadRequest, "project_path and group_name are required")
		return
	}

	route, workspaceNote, err := h.prepareProjectRoute(r.Context(), params.ProjectPath, params.GroupName, params.Task)
	if err != nil {
		writeToolResponse(w, ToolCallbackResponse{
			Content: fmt.Sprintf("准备项目工作区失败: %v", err),
			IsError: true,
		})
		return
	}

	// Get sender OpenID from stored context.
	// Use session_id as the chatKey to find the sender who triggered this tool call.
	senderOpenID := h.getSender(r.Context(), req.SessionID)
	if senderOpenID == "" {
		// Fallback: try all senders (best effort for backward compat)
		senderOpenID = h.getAnySender()
	}

	// Create group chat
	chatID, err := h.client.CreateGroupChat(r.Context(), params.GroupName, "Pi Agent 项目协作群", []string{senderOpenID})
	if err != nil {
		writeToolResponse(w, ToolCallbackResponse{
			Content: fmt.Sprintf("创建群失败: %v", err),
			IsError: true,
		})
		return
	}

	// Bind route
	h.setRoute(chatID, route)

	// Send welcome message to the new group
	welcome := fmt.Sprintf("👋 项目群已创建！\n%s\n\n请在本群中直接发送消息与 AI Agent 对话。", formatProjectWorkspace(route, workspaceNote))
	_, _ = h.client.SendMessage(r.Context(), chatID, welcome, "")

	// Build result with permission link
	permLink := fmt.Sprintf("https://open.feishu.cn/app/%s/auth?q=im%%3Amessage.group_msg&op_from=pi-go&token_type=tenant", h.appID)
	result := fmt.Sprintf("✅ 项目群创建成功！\n📌 群名: %s\n%s\n🆔 Chat ID: %s", params.GroupName, formatProjectWorkspace(route, workspaceNote), chatID)
	if h.appID != "" {
		result += fmt.Sprintf("\n\n⚠️ 如群内消息需要 @ 机器人，请先开通免 @ 权限（只需开通一次，后续所有群自动生效）：\n%s", permLink)
	}
	writeToolResponse(w, ToolCallbackResponse{Content: result})
}

func writeToolResponse(w http.ResponseWriter, resp ToolCallbackResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeToolError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ToolCallbackResponse{Content: msg, IsError: true})
}
