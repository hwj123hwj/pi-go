package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/earendil-works/pi-go/internal/feishu"
	"github.com/joho/godotenv"
)

func main() {
	// Auto-load .env (ignore error — file may not exist in production)
	_ = godotenv.Load()

	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	piAgentURL := os.Getenv("PI_AGENT_URL")
	workspace := os.Getenv("PI_GO_WORKSPACE")

	if appID == "" || appSecret == "" {
		slog.Error("FEISHU_APP_ID and FEISHU_APP_SECRET are required")
		os.Exit(1)
	}
	if piAgentURL == "" {
		piAgentURL = "http://127.0.0.1:8080"
	}

	slog.Info("starting pi-feishu-bridge",
		"piAgentURL", piAgentURL,
		"workspace", workspace,
	)

	// Create components
	client := feishu.NewClient(appID, appSecret)
	handler := feishu.NewHandler(piAgentURL, client, workspace)

	// Wrap handler for gateway
	msgHandler := func(ctx context.Context, msg feishu.Message) {
		handler.Handle(ctx, msg)
	}

	gateway := feishu.NewGateway(appID, appSecret, msgHandler)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Start gateway (blocks until ctx cancelled)
	if err := gateway.Start(ctx); err != nil {
		slog.Error("gateway stopped", "error", err)
	}

	slog.Info("pi-feishu-bridge exited")
}
