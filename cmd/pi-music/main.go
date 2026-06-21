package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	musicagent "github.com/earendil-works/pi-go/internal/agents/music"
	"github.com/earendil-works/pi-go/internal/app"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/mode"
	"github.com/earendil-works/pi-go/internal/music"
	"github.com/earendil-works/pi-go/internal/music/bilibili"
	"github.com/earendil-works/pi-go/internal/music/netease"
)

func main() {
	cfg := config.Default()

	// Load .env files
	envFile := os.Getenv("PI_GO_ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	_ = config.LoadDotEnv(envFile)
	_ = config.LoadDotEnv(envFile + ".local")
	cfg.LoadFromEnv()

	// Parse flags
	modeFlag := flag.String("mode", "serve", "serve, chat, interactive, or run")
	listen := flag.String("listen", fmt.Sprintf("%s:%d", cfg.Host, cfg.MusicPort), "HTTP listen address")
	input := flag.String("prompt", "", "prompt for run mode")
	sessionFlag := flag.String("session", "", "session ID (empty = new session)")
	flag.Parse()

	// Set log level based on mode
	switch *modeFlag {
	case "interactive", "chat":
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	}

	// Create music dependencies
	neClient := netease.NewClient()
	biliClient := bilibili.NewClient()
	cache := music.NewCache()
	router := music.NewSourceRouter(music.SourceNetease,
		music.NewNetEaseAdapter(neClient),
		music.NewBilibiliAdapter(biliClient),
	)

	// Create App with MusicApplication
	application, err := app.New(app.AppOptions{
		Config:      cfg,
		Application: musicagent.NewMusicApplication(cfg, router, cache),
	})
	if err != nil {
		slog.Error("failed to create app", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	switch *modeFlag {
	case "interactive", "chat":
		sess, err := application.LoadOrCreateSession(context.Background(), *sessionFlag)
		must(err)
		must(mode.NewInteractiveMode(sess, nil, application).Run(context.Background()))

	case "run":
		if *input == "" {
			fmt.Fprintln(os.Stderr, "run mode requires -prompt flag")
			os.Exit(1)
		}
		sess, err := application.LoadOrCreateSession(context.Background(), *sessionFlag)
		must(err)
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()
		must(mode.NewPrintMode(sess).Run(ctx, *input))

	case "serve":
		// Set up music audio proxy routes
		musicMux := http.NewServeMux()
		musicHandler := music.NewHandler(router, cache)
		musicHandler.RegisterRoutes(musicMux)

		serveMode := mode.NewServeMode(application, nil)
		serveMode.SetExtraRoutes(musicMux)

		slog.Info("starting pi-music server", "listen", *listen)
		if err := serveMode.Run(*listen); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *modeFlag)
		os.Exit(2)
	}
}

func must(err error) {
	if err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}
