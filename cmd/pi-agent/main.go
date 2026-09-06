package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hwj123hwj/pi-go/internal/agents/coding"
	"github.com/hwj123hwj/pi-go/internal/agents/coding/commands"
	kbapp "github.com/hwj123hwj/pi-go/internal/agents/kb"
	musicapp "github.com/hwj123hwj/pi-go/internal/agents/music"
	"github.com/hwj123hwj/pi-go/internal/app"
	"github.com/hwj123hwj/pi-go/sdk/config"
	"github.com/hwj123hwj/pi-go/internal/mode"
	music "github.com/hwj123hwj/pi-go/internal/music"
	"github.com/hwj123hwj/pi-go/internal/music/bilibili"
	"github.com/hwj123hwj/pi-go/internal/music/netease"
	userprofile "github.com/hwj123hwj/pi-go/internal/profile"
	"github.com/hwj123hwj/pi-go/sdk/runtime"
	"github.com/hwj123hwj/pi-go/internal/scheduler"
	"github.com/hwj123hwj/pi-go/sdk/slashcmd"
	"github.com/hwj123hwj/pi-go/sdk/tools"
	"github.com/hwj123hwj/pi-go/internal/tui"
)

// version is the build version, injected via -ldflags during release builds.
var version = "dev"

func main() {
	cfg := config.Default()

	// Version flag (injectable via -ldflags "-X main.version=...")
	versionFlag := flag.Bool("version", false, "Print version and exit")

	// Config file flag (YAML, loaded before .env and env vars)
	configFile := flag.String("config", "", "Path to YAML config file (e.g. pi-go.yaml)")

	// Load .env files (ignore if missing)
	// PI_GO_ENV_FILE allows custom .env path (e.g. for packaged desktop app)
	envFile := os.Getenv("PI_GO_ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	_ = config.LoadDotEnv(envFile)
	_ = config.LoadDotEnv(envFile + ".local")

	// Load YAML config first (lowest priority), then env vars override
	if *configFile == "" {
		// Auto-detect pi-go.yaml in CWD or ~/.pi-go/
		if _, err := os.Stat("pi-go.yaml"); err == nil {
			*configFile = "pi-go.yaml"
		} else if home, err := os.UserHomeDir(); err == nil {
			p := filepath.Join(home, ".pi-go", "config.yaml")
			if _, err := os.Stat(p); err == nil {
				*configFile = p
			}
		}
	}
	if *configFile != "" {
		if err := cfg.LoadFromYAML(*configFile); err != nil {
			slog.Warn("failed to load config file", "path", *configFile, "error", err)
		} else {
			slog.Info("loaded config file", "path", *configFile)
		}
	}

	cfg.LoadFromEnv()

	// Handle --version
	if *versionFlag {
		fmt.Printf("pi-go %s\n", version)
		return
	}

	// Parse remaining flags
	modeFlag := flag.String("mode", "run", "run, chat, interactive, or serve")
	listen := flag.String("listen", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), "HTTP listen address")
	input := flag.String("prompt", "hello", "prompt for run mode")
	sessionFlag := flag.String("session", "", "session ID (empty = new session)")
	skillDir := flag.String("skill-dir", "", "directory containing skills (SKILL.md files)")
	legacyTUI := flag.Bool("legacy", false, "Use legacy linear CLI instead of Bubble Tea TUI")
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

	// Propagate version to mode package
	mode.SetVersion(version)

	// Create music dependencies
	neClient := netease.NewClient()
	biliClient := bilibili.NewClient()
	musicCache := music.NewCache()
	musicRouter := music.NewSourceRouter(music.SourceNetease,
		music.NewNetEaseAdapter(neClient),
		music.NewBilibiliAdapter(biliClient),
	)

	// Create unified user profile (shared across agents — but each agent only
	// sees categories relevant to its domain via SummaryForCategories)
	profilePath := filepath.Join(cfg.DataDir, "user_profile.json")
	userProfile := userprofile.NewStore(profilePath)

	// Resolve KB repo path from config (default: ~/agent-lessons)
	kbRepoPath := cfg.KBRepoPath
	if kbRepoPath == "" {
		homeDir, _ := os.UserHomeDir()
		kbRepoPath = homeDir + "/agent-lessons"
	}

	// Create App (thin assembly layer) with coding, music, and kb applications.
	// Coding agent does NOT receive the profile (it relies on .llm-wiki + project context).
	// Music agent receives music + general categories only.
	// KB agent receives the full profile (it's the second brain).
	application, err := app.New(app.AppOptions{
		Config:      cfg,
		SkillDirs:   skillDirs(*skillDir),
		Application: coding.NewCodingApplication(cfg),
		Profile:     userProfile,
		Applications: map[string]runtime.Application{
			"coding": coding.NewCodingApplication(cfg),
			"music":  musicapp.NewMusicApplication(cfg, musicRouter, musicCache, userProfile),
			"kb":     kbapp.NewKBApplicationWithProfile(cfg, kbRepoPath, userProfile),
		},
	})
	if err != nil {
		slog.Error("failed to create app", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	// Ensure LSP processes are cleaned up on exit
	defer tools.ResetLSPManager()

	switch *modeFlag {
	case "interactive", "chat":
		sess, err := application.LoadOrCreateSession(context.Background(), *sessionFlag)
		must(err)
		cmds := buildSlashRegistry(application.LoopManager())
		if *legacyTUI {
			must(mode.NewInteractiveMode(sess, cmds, application).Run(context.Background()))
		} else {
			must(tui.Run(sess, cmds, application))
		}

	case "run":
		sess, err := application.LoadOrCreateSession(context.Background(), *sessionFlag)
		must(err)
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()
		must(mode.NewPrintMode(sess).Run(ctx, *input))

	case "serve":
		cmds := buildSlashRegistry(application.LoopManager())
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
func buildSlashRegistry(loopMgr *scheduler.LoopManager) *slashcmd.Registry {
	registry := slashcmd.NewRegistry()
	coding.RegisterCommands(registry)
	if loopMgr != nil {
		commands.RegisterLoopCommands(registry, loopMgr)
	}
	commands.RegisterTaskCommands(registry)
	commands.RegisterUndoCommands(registry)
	commands.RegisterFeishuCommands(registry)
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
