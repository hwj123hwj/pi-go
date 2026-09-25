package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/hwj123hwj/pi-go/internal/app"
	"github.com/hwj123hwj/pi-go/sdk/runtime"
	"github.com/hwj123hwj/pi-go/sdk/workflow"
)

// ─── app 适配器：AgentSession → workflow.Runner ──────────────────────────────

// appRunnerFactory 为每个工作流步骤派生独立 AgentSession（上下文隔离）。
// 会话正常持久化，可在 /sessions 里回看每步的完整执行过程。
type appRunnerFactory struct{ app *app.App }

func (f *appRunnerFactory) NewRunner(ctx context.Context, opts workflow.RunnerOptions) (workflow.Runner, error) {
	cfg := f.app.Config()
	if opts.Model != "" {
		switch cfg.Provider {
		case "openai":
			cfg.OpenAIModel = opts.Model
		case "anthropic":
			cfg.AnthropicModel = opts.Model
		}
	}
	sess, err := f.app.SessionStore().Create(ctx, runtime.AgentSessionOptions{Config: cfg}, f.app.SessionDepsWithApp(""))
	if err != nil {
		return nil, err
	}
	return &agentSessionRunner{sess: sess}, nil
}

type agentSessionRunner struct{ sess *runtime.AgentSession }

func (r *agentSessionRunner) Prompt(ctx context.Context, prompt string) (string, error) {
	msg, err := r.sess.Prompt(ctx, prompt)
	if err != nil {
		return "", err
	}
	return msg.Text, nil
}

// Close 无操作：会话由 SessionRegistry 持有并落盘，供事后回看。
func (r *agentSessionRunner) Close() error { return nil }

// ─── Server 集成 ─────────────────────────────────────────────────────────────

// workflowRegistry 惰性构建运行注册表；测试可通过 SetWorkflowRunnerFactory 注入 fake。
func (s *Server) workflowRegistry() *workflow.Registry {
	s.wfMu.Lock()
	defer s.wfMu.Unlock()
	if s.wfReg == nil {
		factory := s.wfFactory
		if factory == nil {
			factory = &appRunnerFactory{app: s.app}
		}
		s.wfReg = workflow.NewRegistry(&workflow.Engine{
			Factory: factory,
			Dir:     filepath.Join(s.app.Config().DataDir, "workflows"),
		})
	}
	return s.wfReg
}

// SetWorkflowRunnerFactory 注入自定义 RunnerFactory（测试用）；须在首次访问 /workflows 前调用。
func (s *Server) SetWorkflowRunnerFactory(f workflow.RunnerFactory) {
	s.wfMu.Lock()
	defer s.wfMu.Unlock()
	s.wfFactory = f
	s.wfReg = nil
}

func (s *Server) registerWorkflowRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /workflows", s.startWorkflow)
	mux.HandleFunc("GET /workflows", s.listWorkflows)
	mux.HandleFunc("GET /workflows/{id}", s.getWorkflow)
	mux.HandleFunc("POST /workflows/{id}/cancel", s.cancelWorkflow)
	mux.HandleFunc("POST /workflows/{id}/approve", s.approveWorkflow)
	mux.HandleFunc("POST /workflows/{id}/reject", s.rejectWorkflow)
}

// startWorkflow 启动一次运行。请求体两种形式：
//   - YAML 原文（Content-Type: text/yaml | application/x-yaml | application/yaml | text/plain）
//   - JSON {"yaml": "...", "vars": {...}}（vars 覆盖 spec.vars）
func (s *Server) startWorkflow(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	var vars map[string]any
	var yamlSrc []byte
	switch r.Header.Get("Content-Type") {
	case "application/json":
		var req struct {
			YAML string         `json:"yaml"`
			Vars map[string]any `json:"vars"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		yamlSrc, vars = []byte(req.YAML), req.Vars
	default:
		yamlSrc = body
	}

	spec, err := workflow.ParseSpec(yamlSrc)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	runID, err := s.workflowRegistry().Start(spec, vars)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start workflow: "+err.Error())
		return
	}
	slog.Info("workflow started", "run", runID, "name", spec.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"run_id": runID, "status": workflow.StatusRunning})
}

func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
	runs, err := s.workflowRegistry().List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"runs": runs})
}

func (s *Server) getWorkflow(w http.ResponseWriter, r *http.Request) {
	meta, events, err := s.workflowRegistry().Get(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"meta": meta, "events": events})
}

func (s *Server) cancelWorkflow(w http.ResponseWriter, r *http.Request) {
	if err := s.workflowRegistry().Cancel(r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "cancelling"})
}

// gateRequest 可选指定 {"step": "..."}；省略时要求恰好一个等待门。
type gateRequest struct {
	Step string `json:"step"`
}

func (s *Server) approveWorkflow(w http.ResponseWriter, r *http.Request) { s.resolveGateHTTP(w, r, true) }
func (s *Server) rejectWorkflow(w http.ResponseWriter, r *http.Request)  { s.resolveGateHTTP(w, r, false) }

func (s *Server) resolveGateHTTP(w http.ResponseWriter, r *http.Request, approve bool) {
	var req gateRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // 允许空 body

	var err error
	if approve {
		err = s.workflowRegistry().Approve(r.PathValue("id"), req.Step)
	} else {
		err = s.workflowRegistry().Reject(r.PathValue("id"), req.Step)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	action := "rejected"
	if approve {
		action = "approved"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": action})
}
