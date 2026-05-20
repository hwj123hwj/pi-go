package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/session"
)

type Server struct {
	agent   *agent.Agent
	dataDir string
}

type ChatRequest struct {
	Prompt     string `json:"prompt"`
	SessionID  string `json:"session_id,omitempty"`  // 可选：指定会话
	Model      string `json:"model,omitempty"`
	MaxTurns   int    `json:"max_turns,omitempty"`
}

type ChatResponse struct {
	Text      string        `json:"text"`
	ToolCalls []ai.ToolCall `json:"tool_calls,omitempty"`
	SessionID string        `json:"session_id,omitempty"`
}

type SessionResponse struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`
	MessageCount int  `json:"message_count"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func New(ag *agent.Agent) *Server {
	dataDir := "./data"
	return &Server{agent: ag, dataDir: dataDir}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /chat", s.chat)
	mux.HandleFunc("POST /chat/stream", s.chatStream)
	mux.HandleFunc("GET /sessions", s.listSessions)
	mux.HandleFunc("POST /sessions", s.createSession)
	mux.HandleFunc("GET /sessions/{id}/messages", s.getSessionMessages)
	mux.HandleFunc("DELETE /sessions/{id}", s.deleteSession)
	mux.HandleFunc("GET /tools", s.listTools)
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ─── POST /chat ──────────────────────────────────────────────────────────────

func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	assistant, err := s.agent.Prompt(ctx, ai.NewTextUserMessage(req.Prompt))
	if err != nil {
		slog.Error("chat failed", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ChatResponse{Text: assistant.Text, ToolCalls: assistant.ToolCalls})
}

// ─── POST /chat/stream ──────────────────────────────────────────────────────

func (s *Server) chatStream(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	// 设置 SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	stream, err := s.agent.PromptStream(ctx, ai.NewTextUserMessage(req.Prompt))
	if err != nil {
		writeSSE(w, "error", err.Error())
		return
	}

	flusher, canFlush := w.(http.Flusher)

	for event := range stream {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}

		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)

		if canFlush {
			flusher.Flush()
		}
	}

	fmt.Fprintf(w, "event: done\ndata: {}\n\n")
	if canFlush {
		flusher.Flush()
	}
}

// ─── GET /sessions ───────────────────────────────────────────────────────────

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	sessionsDir := filepath.Join(s.dataDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]SessionResponse{})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var sessions []SessionResponse
	for _, entry := range entries {
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			sessions = append(sessions, SessionResponse{
				ID:        entry.Name(),
				CreatedAt: info.ModTime().Unix(),
			})
		}
	}

	if sessions == nil {
		sessions = []SessionResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

// ─── POST /sessions ──────────────────────────────────────────────────────────

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	id := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	sessionsDir := filepath.Join(s.dataDir, "sessions", id)
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SessionResponse{
		ID:        id,
		CreatedAt: time.Now().Unix(),
	})
}

// ─── GET /sessions/{id}/messages ────────────────────────────────────────────

func (s *Server) getSessionMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	sessionDir := filepath.Join(s.dataDir, "sessions", sessionID)

	// 检查 session 目录是否存在
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	sessionPath := filepath.Join(sessionDir, "session.jsonl")

	storage := session.NewJSONLStorage(sessionPath)
	if err := storage.Init(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer storage.Close()

	sess := session.New(storage)
	messages, err := sess.BuildContext(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type messageEntry struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	var result []messageEntry
	for _, msg := range messages {
		entry := messageEntry{Role: string(msg.Role())}
		switch m := msg.(type) {
		case ai.UserMessage:
			var texts []string
			for _, block := range m.Content {
				if block.Type == "text" {
					texts = append(texts, block.Text)
				}
			}
			entry.Content = joinTexts(texts)
		case ai.AssistantMessage:
			entry.Content = m.Text
		case ai.ToolResultMessage:
			entry.Content = m.Content
		}
		result = append(result, entry)
	}

	if result == nil {
		result = []messageEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// ─── DELETE /sessions/{id} ──────────────────────────────────────────────────

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	sessionDir := filepath.Join(s.dataDir, "sessions", sessionID)
	if err := os.RemoveAll(sessionDir); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// ─── GET /tools ──────────────────────────────────────────────────────────────

func (s *Server) listTools(w http.ResponseWriter, r *http.Request) {
	// 通过 agent 获取工具定义 - 使用简单的方式
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tools": []string{"bash", "read", "write", "edit", "grep", "find", "ls"},
	})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}

func writeSSE(w http.ResponseWriter, eventType, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func joinTexts(texts []string) string {
	result := ""
	for i, t := range texts {
		if i > 0 {
			result += "\n"
		}
		result += t
	}
	return result
}
