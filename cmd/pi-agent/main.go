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
		assistant, err := ag.Prompt(ctx, ai.UserMessage{Content: *input})
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
	registry.Register(providers.NewMockProvider())
	toolList := []agent.Tool{tools.NewBashTool(), tools.NewReadTool(), tools.NewWriteTool()}
	cwd, _ := os.Getwd()
	return agent.New(agent.Options{
		Model:    ai.Model{ID: "mock", Name: "Mock", Provider: "mock", ContextWindow: 128000, MaxTokens: 4096},
		Registry: registry,
		System:   prompt.BuildSystemPrompt(prompt.Options{CWD: cwd, Tools: toolList}),
		Tools:    toolList,
		MaxTurns: cfg.MaxTurns,
	})
}

func runChat(ag *agent.Agent) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("pi-go chat mode. Ctrl-D to exit.")
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		assistant, err := ag.Prompt(ctx, ai.UserMessage{Content: scanner.Text()})
		cancel()
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println(assistant.Text)
	}
}
