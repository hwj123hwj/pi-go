package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/app"
	"github.com/earendil-works/pi-go/internal/runtime"
	"github.com/earendil-works/pi-go/internal/slashcmd"
	"github.com/earendil-works/pi-go/internal/web"
)

// Server provides HTTP REST + SSE endpoints for the agent.
// It routes requests to AgentSessions via the App's SessionRegistry.
type Server struct {
	app           *app.App
	slashCmds     *slashcmd.Registry
	externalTools []agent.ExternalToolDef
	toolMu        sync.Mutex
}

// ChatRequest is the request body for chat endpoints.
type ChatRequest struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatResponse is the response for non-streaming chat.
type ChatResponse struct {
	Text      string        `json:"text"`
	ToolCalls []ai.ToolCall `json:"tool_calls,omitempty"`
	SessionID string        `json:"session_id,omitempty"`
}

// SessionResponse is the response for session metadata.
type SessionResponse struct {
	ID           string `json:"id"`
	CreatedAt    int64  `json:"created_at"`
	MessageCount int    `json:"message_count"`
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// New creates a new Server backed by the given App and slash command registry.
func New(application *app.App, slashCmds *slashcmd.Registry) *Server {
	return &Server{app: application, slashCmds: slashCmds}
}

// Handler returns the HTTP handler with all routes and middleware.
func (s *Server) Handler() http.Handler {
	// REST API routes with middleware
	restMux := http.NewServeMux()
	restMux.HandleFunc("GET /health", s.health)
	restMux.HandleFunc("POST /chat", s.chat)
	restMux.HandleFunc("POST /chat/stream", s.chatStream)
	restMux.HandleFunc("GET /sessions", s.listSessions)
	restMux.HandleFunc("POST /sessions", s.createSession)
	restMux.HandleFunc("GET /sessions/{id}/messages", s.getSessionMessages)
	restMux.HandleFunc("GET /sessions/{id}/info", s.getSessionInfo)
	restMux.HandleFunc("DELETE /sessions/{id}", s.deleteSession)
	restMux.HandleFunc("POST /sessions/{id}/model", s.switchModel)
	restMux.HandleFunc("GET /models", s.listModels)
	restMux.HandleFunc("GET /tools", s.listTools)
	restMux.HandleFunc("POST /sessions/{id}/compact", s.compactSession)
	restMux.HandleFunc("POST /sessions/{id}/command", s.executeCommand)
	restMux.HandleFunc("POST /tools/register", s.registerTool)

	var restHandler http.Handler = restMux
	restHandler = corsMiddleware(restHandler)
	restHandler = recoveryMiddleware(restHandler)
	restHandler = loggingMiddleware(restHandler)

	// WebSocket route — bypasses all middleware to avoid Hijack issues
	wsHandler := corsMiddleware(http.HandlerFunc(s.handleWebSocket))

	// Top-level mux: combines REST API + Web UI + WebSocket
	topMux := http.NewServeMux()
	topMux.Handle("GET /ws", wsHandler)

	// Register REST API routes (these take precedence over "/" catch-all)
	topMux.Handle("/health", restHandler)
	topMux.Handle("/chat", restHandler)
	topMux.Handle("/sessions", restHandler)
	topMux.Handle("/sessions/", restHandler)
	topMux.Handle("/models", restHandler)
	topMux.Handle("/tools", restHandler)

	// Register web UI routes (serves embedded static files at /)
	web.RegisterRoutes(topMux)

	return topMux
}

// ListenAndServe starts the HTTP server on the given address.
func (s *Server) ListenAndServe(addr string) error {
	slog.Info("starting pi-go server", "listen", addr)
	return http.ListenAndServe(addr, s.Handler())
}

// ─── GET /health ──────────────────────────────────────────────────────────────

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ─── POST /chat ───────────────────────────────────────────────────────────────

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

	sess, err := s.resolveSession(ctx, req.SessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	assistant, err := sess.Prompt(ctx, req.Prompt)
	if err != nil {
		if errors.Is(err, agent.ErrAgentBusy) {
			writeError(w, http.StatusConflict, "agent is busy processing another request")
			return
		}
		slog.Error("chat failed", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ChatResponse{
		Text:      assistant.Text,
		ToolCalls: assistant.ToolCalls,
		SessionID: sess.SessionID(),
	})
}

// ─── POST /chat/stream ────────────────────────────────────────────────────────

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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	sess, err := s.resolveSession(ctx, req.SessionID)
	if err != nil {
		writeSSE(w, "error", err.Error())
		return
	}

	writeSSE(w, "session_id", sess.SessionID())

	stream, err := sess.PromptStream(ctx, req.Prompt)
	if err != nil {
		if errors.Is(err, agent.ErrAgentBusy) {
			writeSSE(w, "error", "agent is busy processing another request")
			return
		}
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
}

// ─── GET /sessions ────────────────────────────────────────────────────────────

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	mgr := s.app.SessionManager()
	sessions, err := mgr.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

// ─── POST /sessions ───────────────────────────────────────────────────────────

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	sess, err := s.app.NewSession(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SessionResponse{
		ID:        sess.SessionID(),
		CreatedAt: time.Now().Unix(),
	})
}

// ─── GET /sessions/{id}/messages ──────────────────────────────────────────────

func (s *Server) getSessionMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	// Check if session exists in the session manager
	if !s.app.SessionManager().Exists(sessionID) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	sess, err := s.app.LoadSession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	messages, err := sess.Session().BuildContext(r.Context())
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

// ─── DELETE /sessions/{id} ────────────────────────────────────────────────────

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	mgr := s.app.SessionManager()
	if err := mgr.Delete(sessionID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = s.app.SessionStore().Delete(sessionID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// ─── GET /tools ───────────────────────────────────────────────────────────────

func (s *Server) listTools(w http.ResponseWriter, r *http.Request) {
	toolNames := s.app.ToolNames()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tools": toolNames,
	})
}

// ─── GET /models ───────────────────────────────────────────────────────────────

// ModelInfo holds metadata about a single model.
type ModelInfo struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Name     string `json:"name"`
}

// ModelsResponse is the response for the models list endpoint.
type ModelsResponse struct {
	Models  []ModelInfo      `json:"models"`
	Current *ModelInfo       `json:"current,omitempty"`
}

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	// Build model catalog from known models
	models := []ModelInfo{
		{ID: "deepseek-v4-flash", Provider: "deepv", Name: "DeepSeek V4 Flash"},
		{ID: "glm-5", Provider: "deepv", Name: "GLM-5"},
		{ID: "claude-sonnet-4-6", Provider: "deepv", Name: "Claude Sonnet 4.6"},
	}

	// Determine current model from config
	cfg := s.app.Config()
	var current *ModelInfo
	provider := cfg.Provider
	modelID := ""
	switch provider {
	case "openai":
		modelID = cfg.OpenAIModel
	case "deepv":
		modelID = cfg.DeepVModel
	case "anthropic":
		modelID = cfg.AnthropicModel
	}
	if modelID != "" {
		current = &ModelInfo{ID: modelID, Provider: provider, Name: modelID}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ModelsResponse{Models: models, Current: current})
}

// ─── GET /sessions/{id}/info ──────────────────────────────────────────────────

func (s *Server) getSessionInfo(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	if !s.app.SessionManager().Exists(sessionID) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	sess, err := s.app.LoadSession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	provider, modelID := sess.ModelInfo()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":        sess.SessionID(),
		"provider":  provider,
		"model":     modelID,
	})
}

// ─── POST /sessions/{id}/model ────────────────────────────────────────────────

// SwitchModelRequest is the request body for switching a session's model.
type SwitchModelRequest struct {
	Model    string `json:"model"`
	Provider string `json:"provider,omitempty"` // Optional: change provider along with model
}

func (s *Server) switchModel(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	var req SwitchModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	sess, err := s.app.LoadSession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if err := sess.SwitchModel(r.Context(), req.Model, req.Provider); err != nil {
		writeError(w, http.StatusInternalServerError, "switch model failed: "+err.Error())
		return
	}

	provider, modelID := sess.ModelInfo()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"provider": provider,
		"model":    modelID,
	})
}

// ─── POST /sessions/{id}/compact ──────────────────────────────────────────────

// CompactRequest is the request body for context compaction.
type CompactRequest struct {
	CustomInstructions string `json:"custom_instructions,omitempty"`
}

// CompactResponse is the response for context compaction.
type CompactResponse struct {
	Summary     string `json:"summary"`
	TrimmedFrom int    `json:"trimmed_from"`
	TrimmedTo   int    `json:"trimmed_to"`
}

func (s *Server) compactSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	var req CompactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body
		req = CompactRequest{}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	sess, err := s.app.LoadSession(ctx, sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	summary, trimmedFrom, trimmedTo, err := sess.Compact(ctx, req.CustomInstructions)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "compact failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(CompactResponse{
		Summary:     summary,
		TrimmedFrom: trimmedFrom,
		TrimmedTo:   trimmedTo,
	})
}

// ─── POST /sessions/{id}/command ──────────────────────────────────────────────

// CommandRequest is the request body for executing a slash command.
type CommandRequest struct {
	Command string `json:"command"` // e.g. "/model claude-sonnet-4-6"
}

// CommandResponse is the response for slash command execution.
type CommandResponse struct {
	Output      string `json:"output"`
	ShouldQuery bool   `json:"should_query"`
	QueryPrompt string `json:"query_prompt,omitempty"`
}

func (s *Server) executeCommand(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	var req CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	if s.slashCmds == nil {
		writeError(w, http.StatusNotImplemented, "slash commands not available in server mode")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	sess, err := s.app.LoadSession(ctx, sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	cmdCtx := slashcmd.Context{
		Ctx:     ctx,
		Session: sess,
		App:     s.app,
	}

	result, err := s.slashCmds.Execute(cmdCtx, req.Command)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := CommandResponse{
		Output: result.Output,
	}
	if result.ShouldQuery {
		resp.ShouldQuery = true
		resp.QueryPrompt = "Start working..."
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ─── POST /tools/register ────────────────────────────────────────────────────

func (s *Server) registerTool(w http.ResponseWriter, r *http.Request) {
	var def agent.ExternalToolDef
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if def.Name == "" {
		writeError(w, http.StatusBadRequest, "tool name is required")
		return
	}
	if def.CallbackURL == "" {
		writeError(w, http.StatusBadRequest, "callback_url is required")
		return
	}

	// Validate the tool can be constructed
	if _, err := agent.NewExternalTool(def); err != nil {
		writeError(w, http.StatusBadRequest, "invalid tool: "+err.Error())
		return
	}

	s.toolMu.Lock()
	// Replace if same name exists
	updated := false
	for i, existing := range s.externalTools {
		if existing.Name == def.Name {
			s.externalTools[i] = def
			updated = true
			break
		}
	}
	if !updated {
		s.externalTools = append(s.externalTools, def)
	}
	s.syncToApp()
	s.toolMu.Unlock()

	if updated {
		slog.Info("updated external tool", "name", def.Name, "callback", def.CallbackURL)
	} else {
		slog.Info("registered external tool", "name", def.Name, "callback", def.CallbackURL)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "name": def.Name})
}

// ExternalTools returns all registered external tool definitions.
func (s *Server) ExternalTools() []agent.ExternalToolDef {
	s.toolMu.Lock()
	defer s.toolMu.Unlock()
	result := make([]agent.ExternalToolDef, len(s.externalTools))
	copy(result, s.externalTools)
	return result
}

// syncToApp pushes external tools to the App (must be called with toolMu held).
func (s *Server) syncToApp() {
	defs := make([]agent.ExternalToolDef, len(s.externalTools))
	copy(defs, s.externalTools)
	s.app.SetExternalTools(defs)
}

// ─── session resolution ──────────────────────────────────────────────────────

// resolveSession gets an existing session or creates a new one.
func (s *Server) resolveSession(ctx context.Context, sessionID string) (*runtime.AgentSession, error) {
	return s.app.LoadOrCreateSession(ctx, sessionID)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

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

// ─── middleware ───────────────────────────────────────────────────────────────

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration", time.Since(start).Round(time.Millisecond),
		)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("handler panic", "error", err, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("internal error: %v", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements the http.Hijacker interface so WebSocket upgrades work
// through the logging middleware wrapper.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rw.ResponseWriter.(http.Hijacker).Hijack()
}
