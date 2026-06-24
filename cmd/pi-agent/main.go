package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/hwj123hwj/pi-go/internal/agents/coding"
	kbapp "github.com/hwj123hwj/pi-go/internal/agents/kb"
	musicapp "github.com/hwj123hwj/pi-go/internal/agents/music"
	"github.com/hwj123hwj/pi-go/internal/app"
	"github.com/hwj123hwj/pi-go/internal/config"
	"github.com/hwj123hwj/pi-go/internal/mode"
	music "github.com/hwj123hwj/pi-go/internal/music"
	"github.com/hwj123hwj/pi-go/internal/music/bilibili"
	"github.com/hwj123hwj/pi-go/internal/music/netease"
	"github.com/hwj123hwj/pi-go/internal/runtime"
	"github.com/hwj123hwj/pi-go/internal/slashcmd"
)

func main() {
	cfg := config.Default()

	// Load .env files (ignore if missing)
	// PI_GO_ENV_FILE allows custom .env path (e.g. for packaged desktop app)
	envFile := os.Getenv("PI_GO_ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	_ = config.LoadDotEnv(envFile)
	_ = config.LoadDotEnv(envFile + ".local")
	cfg.LoadFromEnv()

	// Parse flags
	modeFlag := flag.String("mode", "run", "run, chat, interactive, or serve")
	listen := flag.String("listen", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), "HTTP listen address")
	input := flag.String("prompt", "hello", "prompt for run mode")
	sessionFlag := flag.String("session", "", "session ID (empty = new session)")
	skillDir := flag.String("skill-dir", "", "directory containing skills (SKILL.md files)")
	flag.Parse()

	// Sync the actual listen port back to config so MusicApplication
	// generates correct audio proxy URLs (the desktop app uses random ports).
	if _, portStr, err := net.SplitHostPort(*listen); err == nil {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = p
		}
	}
	if host, _, err := net.SplitHostPort(*listen); err == nil && host != "" {
		cfg.Host = host
	}

	// Set log level based on mode
	switch *modeFlag {
	case "interactive", "chat":
		// Suppress INFO logs in TUI mode for cleaner output
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	default:
		// Keep default INFO level for other modes
	}

	// Create music dependencies
	neClient := netease.NewClient()
	biliClient := bilibili.NewClient()
	musicCache := music.NewCache()
	musicRouter := music.NewSourceRouter(music.SourceNetease,
		music.NewNetEaseAdapter(neClient),
		music.NewBilibiliAdapter(biliClient),
	)

	// Determine agent-lessons repo path (default: ~/agent-lessons)
	homeDir, _ := os.UserHomeDir()
	kbRepoPath := os.Getenv("PI_GO_KB_REPO_PATH")
	if kbRepoPath == "" {
		kbRepoPath = homeDir + "/agent-lessons"
	}

	// Create App (thin assembly layer) with coding, music, and kb applications
	application, err := app.New(app.AppOptions{
		Config:      cfg,
		SkillDirs:   skillDirs(*skillDir),
		Application: coding.NewCodingApplication(cfg),
		Applications: map[string]runtime.Application{
			"coding": coding.NewCodingApplication(cfg),
			"music":  musicapp.NewMusicApplication(cfg, musicRouter, musicCache),
			"kb":     kbapp.NewKBApplication(cfg, kbRepoPath),
		},
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
		cmds := buildSlashRegistry()
		must(mode.NewInteractiveMode(sess, cmds, application).Run(context.Background()))

	case "run":
		sess, err := application.LoadOrCreateSession(context.Background(), *sessionFlag)
		must(err)
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()
		must(mode.NewPrintMode(sess).Run(ctx, *input))

	case "serve":
		cmds := buildSlashRegistry()
		// Music audio proxy routes
		musicHandler := music.NewHandler(musicRouter, musicCache)
		extraMux := http.NewServeMux()
		musicHandler.RegisterRoutes(extraMux)
		srv := mode.NewServeMode(application, cmds)
		srv.SetExtraRoutes(extraMux)
		slog.Info("starting pi-go server", "listen", *listen)
		if err := srv.Run(*listen); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *modeFlag)
		os.Exit(2)
	}
}

// buildSlashRegistry creates the slash command registry with built-in commands.
func buildSlashRegistry() *slashcmd.Registry {
	registry := slashcmd.NewRegistry()
	coding.RegisterCommands(registry)
	return registry
}

// skillDirs returns the skill directory list from a flag value.
func skillDirs(flag string) []string {
	if flag != "" {
		return []string{flag}
	}
	return nil
}

// must panics on error.
func must(err error) {
	if err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}
