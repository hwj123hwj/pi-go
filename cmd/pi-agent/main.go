package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/earendil-works/pi-go/internal/app"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/mode"
	"github.com/earendil-works/pi-go/internal/slashcmd"
)

func main() {
	cfg := config.Default()

	// Load .env files (ignore if missing)
	_ = config.LoadDotEnv(".env")
	_ = config.LoadDotEnv(".env.local")
	cfg.LoadFromEnv()

	// Parse flags
	modeFlag := flag.String("mode", "run", "run, chat, interactive, or serve")
	listen := flag.String("listen", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), "HTTP listen address")
	input := flag.String("prompt", "hello", "prompt for run mode")
	sessionFlag := flag.String("session", "", "session ID (empty = new session)")
	skillDir := flag.String("skill-dir", "", "directory containing skills (SKILL.md files)")
	flag.Parse()

	// Create App (thin assembly layer)
	application, err := app.New(app.AppOptions{
		Config:    cfg,
		SkillDirs: skillDirs(*skillDir),
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
		slog.Info("starting pi-go server", "listen", *listen)
		if err := mode.NewServeMode(application).Run(*listen); err != nil {
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
	slashcmd.RegisterBuiltins(registry)
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
