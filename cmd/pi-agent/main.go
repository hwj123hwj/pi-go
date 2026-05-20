package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/ai/providers"
	"github.com/earendil-works/pi-go/internal/compaction"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/prompt"
	"github.com/earendil-works/pi-go/internal/server"
	"github.com/earendil-works/pi-go/internal/session"
	"github.com/earendil-works/pi-go/internal/skill"
	"github.com/earendil-works/pi-go/internal/tools"
)

func main() {
	cfg := config.Default()

	// 加载 .env 文件（不存在则忽略）
	_ = config.LoadDotEnv(".env")
	_ = config.LoadDotEnv(".env.local")
	cfg.LoadFromEnv()

	mode := flag.String("mode", "run", "run, chat, or serve")
	listen := flag.String("listen", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), "HTTP listen address")
	input := flag.String("prompt", "hello", "prompt for run mode")
	sessionFlag := flag.String("session", "", "session ID for chat mode (empty = new session)")
	skillDir := flag.String("skill-dir", "", "directory containing skills (SKILL.md files)")
	flag.Parse()

	ag := buildAgent(cfg, *sessionFlag, *skillDir)

	switch *mode {
	case "serve":
		srv := server.New(ag)
		slog.Info("starting pi-go server", "listen", *listen)
		if err := http.ListenAndServe(*listen, srv.Handler()); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	case "chat":
		runChat(ag)
	case "run":
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()
		assistant, err := ag.Prompt(ctx, ai.NewTextUserMessage(*input))
		if err != nil {
			slog.Error("run failed", "error", err)
			os.Exit(1)
		}
		fmt.Println(assistant.Text)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		os.Exit(2)
	}
}

func buildAgent(cfg config.Config, sessionID string, skillDir string) *agent.Agent {
	registry := providers.NewRegistry()

	// 注册 MockProvider（始终可用）
	registry.Register(providers.NewMockProvider())

	// 根据配置注册真实 Provider
	switch cfg.Provider {
	case "anthropic":
		if cfg.AnthropicAPIKey != "" {
			registry.Register(providers.NewAnthropicProvider(cfg.AnthropicAPIKey, cfg.AnthropicBaseURL))
			slog.Info("registered anthropic provider", "model", cfg.AnthropicModel, "base_url", cfg.AnthropicBaseURL)
		} else {
			slog.Warn("anthropic provider selected but ANTHROPIC_API_KEY is empty, falling back to mock")
		}
	case "openai":
		if cfg.OpenAIAPIKey != "" {
			registry.Register(providers.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL))
			slog.Info("registered openai provider", "model", cfg.OpenAIModel, "base_url", cfg.OpenAIBaseURL)
		} else {
			slog.Warn("openai provider selected but OPENAI_API_KEY is empty, falling back to mock")
		}
	default:
		slog.Info("using mock provider (set PI_GO_PROVIDER=anthropic or openai for real LLM)")
	}

	cwd, _ := os.Getwd()

	toolList := []agent.Tool{
		tools.NewBashTool(cwd),
		tools.NewReadTool(),
		tools.NewWriteTool(),
		tools.NewEditTool(),
		tools.NewGrepTool(),
		tools.NewFindTool(),
		tools.NewLsTool(),
	}

	// 确定实际使用的 model 和 provider name
	modelID := cfg.AnthropicModel
	providerName := cfg.Provider
	if providerName == "openai" {
		modelID = cfg.OpenAIModel
	}
	if providerName == "mock" || modelID == "" {
		modelID = "mock"
		providerName = "mock"
	}

	model := ai.Model{
		ID:            modelID,
		Name:          modelID,
		Provider:      providerName,
		ContextWindow: contextWindowForModel(modelID),
		MaxTokens:     4096,
	}

	// 会话持久化（可选）
	var sess *session.Session
	if sessionID != "" || cfg.SessionFile != "" {
		sessionPath := cfg.SessionFile
		if sessionID != "" {
			sessionPath = fmt.Sprintf("./data/sessions/%s/session.jsonl", sessionID)
		}
		storage := session.NewJSONLStorage(sessionPath)
		if err := storage.Init(); err != nil {
			slog.Warn("failed to init session storage", "error", err, "path", sessionPath)
		} else {
			sess = session.New(storage)
			slog.Info("session initialized", "path", sessionPath)
		}
	}

	// 上下文压缩配置
	compactionSettings := compaction.DefaultSettings()
	var summarizeFunc compaction.SummarizeFunc
	if providerName != "mock" {
		summarizeFunc = compaction.LLMSummarizer(registry, model)
	}

	// 加载项目上下文文件（CLAUDE.md 等）
	contextFiles := prompt.LoadProjectContextFiles(cwd, "")
	if len(contextFiles) > 0 {
		for _, cf := range contextFiles {
			slog.Info("loaded context file", "path", cf.Path, "size", len(cf.Content))
		}
	}

	// 加载技能
	var skills []skill.Skill
	skillDirs := []string{}
	if skillDir != "" {
		skillDirs = append(skillDirs, skillDir)
	}
	// 默认也查找 .claude/skills 目录
	defaultSkillDir := filepath.Join(cwd, ".claude", "skills")
	if fi, err := os.Stat(defaultSkillDir); err == nil && fi.IsDir() {
		skillDirs = append(skillDirs, defaultSkillDir)
	}
	// 也查找 home 目录下的全局技能
	homeSkillDir := filepath.Join(homeDir(), ".claude", "skills")
	if fi, err := os.Stat(homeSkillDir); err == nil && fi.IsDir() {
		skillDirs = append(skillDirs, homeSkillDir)
	}

	if len(skillDirs) > 0 {
		result := skill.LoadFromDirs(skillDirs...)
		skills = result.Skills
		for _, s := range skills {
			slog.Info("loaded skill", "name", s.Name, "path", s.FilePath)
		}
		for _, d := range result.Diagnostics {
			slog.Warn("skill diagnostic", "code", d.Code, "message", d.Message, "path", d.Path)
		}
	}

	// 构建系统提示
	systemPrompt := prompt.BuildSystemPrompt(prompt.Options{
		CWD:          cwd,
		Tools:        toolList,
		ContextFiles: contextFiles,
		Skills:       skills,
	})

	return agent.New(agent.Options{
		Model:              model,
		Registry:           registry,
		System:             systemPrompt,
		Tools:              toolList,
		MaxTurns:           cfg.MaxTurns,
		Session:            sess,
		CompactionSettings: compactionSettings,
		SummarizeFunc:      summarizeFunc,
	})
}

func runChat(ag *agent.Agent) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("pi-go chat mode. Type your message and press Enter. Ctrl-D to exit.")
	fmt.Println()

	for {
		fmt.Print("You> ")
		if !scanner.Scan() {
			return
		}
		input := scanner.Text()
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

		stream, err := ag.PromptStream(ctx, ai.NewTextUserMessage(input))
		if err != nil {
			fmt.Println("error:", err)
			cancel()
			continue
		}

		fmt.Print("Pi> ")
		for event := range stream {
			switch event.Type {
			case agent.StreamEventTextDelta:
				fmt.Print(event.TextDelta)
			case agent.StreamEventToolStart:
				fmt.Printf("\n[tool:%s] ", event.ToolName)
			case agent.StreamEventToolEnd:
				fmt.Print("✓ ")
			case agent.StreamEventCompacted:
				fmt.Printf("\n[compacted: %d chars trimmed] ", len(event.Summary))
			case agent.StreamEventDone:
				// 确保换行
				fmt.Println()
			case agent.StreamEventError:
				fmt.Printf("\nerror: %s\n", event.Error)
			}
		}

		cancel()
		fmt.Println()
	}
}

// homeDir 获取用户 home 目录。
func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return ""
}

// contextWindowForModel 返回模型对应的上下文窗口大小。
func contextWindowForModel(modelID string) int {
	windows := map[string]int{
		// Anthropic
		"claude-3-5-sonnet":   200000,
		"claude-3-5-haiku":    200000,
		"claude-3-opus":       200000,
		"claude-sonnet-4":     200000,
		"claude-sonnet-4-5":   200000,
		// OpenAI
		"gpt-4o":              128000,
		"gpt-4o-mini":         128000,
		"gpt-4-turbo":         128000,
		"gpt-4":               8192,
		"o1":                  200000,
		"o1-mini":             128000,
		"o3-mini":             200000,
	}
	if w, ok := windows[modelID]; ok {
		return w
	}
	return 128000 // 默认值
}
