package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/ai/providers"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/prompt"
	"github.com/earendil-works/pi-go/internal/server"
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
	flag.Parse()

	ag := buildAgent(cfg)

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

func buildAgent(cfg config.Config) *agent.Agent {
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

	toolList := []agent.Tool{tools.NewBashTool(), tools.NewReadTool(), tools.NewWriteTool()}
	cwd, _ := os.Getwd()

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

	return agent.New(agent.Options{
		Model: ai.Model{
			ID:            modelID,
			Name:          modelID,
			Provider:      providerName,
			ContextWindow: 128000,
			MaxTokens:     4096,
		},
		Registry: registry,
		System:   prompt.BuildSystemPrompt(prompt.Options{CWD: cwd, Tools: toolList}),
		Tools:    toolList,
		MaxTurns: cfg.MaxTurns,
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
