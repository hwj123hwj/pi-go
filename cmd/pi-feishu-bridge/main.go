package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hwj123hwj/pi-go/internal/feishu"
	"github.com/joho/godotenv"
)

func main() {
	// Auto-load .env (ignore error — file may not exist in production)
	_ = godotenv.Load()

	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	piAgentURL := os.Getenv("PI_AGENT_URL")
	workspace := os.Getenv("PI_GO_WORKSPACE")
	callbackURL := os.Getenv("BRIDGE_CALLBACK_URL")
	callbackAddr := os.Getenv("BRIDGE_CALLBACK_ADDR")

	if appID == "" || appSecret == "" {
		slog.Error("FEISHU_APP_ID and FEISHU_APP_SECRET are required")
		os.Exit(1)
	}
	if piAgentURL == "" {
		piAgentURL = "http://127.0.0.1:8080"
	}
	if callbackAddr == "" {
		callbackAddr = ":9090"
	}

	slog.Info("starting pi-feishu-bridge",
		"piAgentURL", piAgentURL,
		"workspace", workspace,
		"callbackURL", callbackURL,
	)

	// Create components
	client := feishu.NewClient(appID, appSecret)
	handler := feishu.NewHandler(piAgentURL, appID, client, workspace)

	// Wrap handler for gateway
	msgHandler := func(ctx context.Context, msg feishu.Message) {
		handler.Handle(ctx, msg)
	}

	gateway := feishu.NewGateway(appID, appSecret, client, msgHandler)
	handler.SetGateway(gateway)

	// Start tool callback HTTP server
	if callbackURL != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("/tool-callback", handler.HandleToolCallback)
		go func() {
			slog.Info("starting tool callback server", "addr", callbackAddr)
			if err := http.ListenAndServe(callbackAddr, mux); err != nil && err != http.ErrServerClosed {
				slog.Error("tool callback server stopped", "error", err)
			}
		}()

		// Register tool with pi-agent
		fullCallbackURL := callbackURL + "/tool-callback"
		if err := feishu.RegisterTool(piAgentURL, fullCallbackURL); err != nil {
			slog.Warn("failed to register tool (will work without agent tool)", "error", err)
		}
	}

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
