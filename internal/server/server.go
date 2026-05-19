package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/ai"
)

type Server struct {
	agent *agent.Agent
}

type ChatRequest struct {
	Prompt string `json:"prompt"`
}

type ChatResponse struct {
	Text      string        `json:"text"`
	ToolCalls []ai.ToolCall `json:"tool_calls,omitempty"`
}

func New(agent *agent.Agent) *Server {
	return &Server{agent: agent}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /chat", s.chat)
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	assistant, err := s.agent.Prompt(ctx, ai.NewTextUserMessage(req.Prompt))
	if err != nil {
		slog.Error("chat failed", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ChatResponse{Text: assistant.Text, ToolCalls: assistant.ToolCalls})
}
