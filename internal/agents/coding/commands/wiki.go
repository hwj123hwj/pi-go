package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hwj123hwj/pi-go/sdk/slashcmd"
)

// RegisterWikiCommands registers all /wiki sub-commands into the shared registry.
func RegisterWikiCommands(registry *slashcmd.Registry) {
	registry.Register(slashcmd.Command{
		Name:        "wiki",
		Description: "LLM Wiki — init, ingest, query, lint, status, log",
		Handler:     wikiDispatch,
	})
}

// wikiDispatch routes /wiki sub-commands.
func wikiDispatch(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
	parts := strings.SplitN(args, " ", 2)
	sub := parts[0]
	subArgs := ""
	if len(parts) > 1 {
		subArgs = strings.TrimSpace(parts[1])
	}

	switch sub {
	case "init":
		return wikiInit(ctx)
	case "ingest":
		return wikiIngest(ctx, subArgs)
	case "query", "q", "ask":
		return wikiQuery(ctx, subArgs)
	case "lint":
		return wikiLint(ctx)
	case "status":
		return wikiStatus(ctx)
	case "log":
		return wikiLog(ctx)
	default:
		return slashcmd.CommandResult{
			Output: fmt.Sprintf("Unknown wiki subcommand: %s\n\nUsage: /wiki <init|ingest|query|lint|status|log> [args]\n  /wiki init                              — Initialize wiki directory structure\n  /wiki ingest [path]                     — Ingest source file(s) into wiki\n  /wiki query <question>                  — Query the wiki with a question\n  /wiki lint                              — Health check (dead links, orphans, staleness)\n  /wiki status                            — Show wiki statistics\n  /wiki log                               — Show operation history", sub),
		}, nil
	}
}

// wikiInit handles `/wiki init`.
func wikiInit(ctx slashcmd.Context) (slashcmd.CommandResult, error) {
	if isWikiInitialized(ctx) {
		wikiRoot := getWikiRoot(ctx)
		return slashcmd.CommandResult{
			Output: fmt.Sprintf("Wiki already initialized at `%s`\n\nUse `/wiki status` to see current state.", wikiRoot),
		}, nil
	}

	return slashcmd.CommandResult{
		Output:      "Initializing wiki...\n\n" + WikiInitPrompt,
		ShouldQuery: true,
	}, nil
}

// wikiIngest handles `/wiki ingest [path]`.
func wikiIngest(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
	if !isWikiInitialized(ctx) {
		return slashcmd.CommandResult{
			Output: "Wiki not initialized. Run `/wiki init` first.",
		}, nil
	}

	var prompt string
	if args == "" {
		// Ingest all un-ingested files in raw/
		rawDir := filepath.Join(getWikiRoot(ctx), "raw")
		count := countFiles(rawDir)
		if count == 0 {
			return slashcmd.CommandResult{
				Output: fmt.Sprintf("No raw sources found in `%s`.\n\nPlace source files in .llm-wiki/raw/ and run again.", rawDir),
			}, nil
		}
		prompt = fmt.Sprintf("Ingesting all %d raw sources...\n\n%s", count, GetWikiIngestPrompt(""))
	} else {
		prompt = fmt.Sprintf("Ingesting `%s`...\n\n%s", args, GetWikiIngestPrompt(args))
	}

	return slashcmd.CommandResult{
		Output:      prompt,
		ShouldQuery: true,
	}, nil
}

// wikiQuery handles `/wiki query <question>`.
func wikiQuery(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
	if !isWikiInitialized(ctx) {
		return slashcmd.CommandResult{
			Output: "Wiki not initialized. Run `/wiki init` first.",
		}, nil
	}
	if args == "" {
		return slashcmd.CommandResult{
			Output: "Usage: /wiki query <question>\n\nExample: /wiki query \"How does the agent loop work?\"",
		}, nil
	}

	prompt := fmt.Sprintf("Querying wiki: %s\n\n%s", args, GetWikiQueryPrompt(args))
	return slashcmd.CommandResult{
		Output:      prompt,
		ShouldQuery: true,
	}, nil
}

// wikiLint handles `/wiki lint`.
func wikiLint(ctx slashcmd.Context) (slashcmd.CommandResult, error) {
	if !isWikiInitialized(ctx) {
		return slashcmd.CommandResult{
			Output: "Wiki not initialized. Run `/wiki init` first.",
		}, nil
	}

	return slashcmd.CommandResult{
		Output:      "Running wiki health check...\n\n" + WikiLintPrompt,
		ShouldQuery: true,
	}, nil
}

// wikiStatus handles `/wiki status` — pure Go implementation, no AI needed.
func wikiStatus(ctx slashcmd.Context) (slashcmd.CommandResult, error) {
	wikiRoot := getWikiRoot(ctx)
	indexPath := filepath.Join(wikiRoot, "index.md")
	rawDir := filepath.Join(wikiRoot, "raw")
	wikiDir := filepath.Join(wikiRoot, "wiki")

	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return slashcmd.CommandResult{
			Output: fmt.Sprintf("Wiki not initialized at `%s`\n\nRun `/wiki init` to get started.", wikiRoot),
		}, nil
	}

	var b strings.Builder
	b.WriteString("📚 **LLM Wiki Status**\n\n")
	b.WriteString(fmt.Sprintf("Path: `%s`\n", wikiRoot))

	// Count wiki pages
	pageCount := 0
	sourceCount := 0
	entityCount := 0
	conceptCount := 0
	if entries, err := os.ReadDir(wikiDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			pageCount++
			if strings.HasPrefix(e.Name(), "source-") {
				sourceCount++
			} else if strings.HasPrefix(e.Name(), "concept-") {
				conceptCount++
			} else if strings.HasSuffix(e.Name(), ".md") {
				entityCount++
			}
		}
	}

	b.WriteString(fmt.Sprintf("Wiki pages: %d (%d sources, %d entities, %d concepts)\n",
		pageCount, sourceCount, entityCount, conceptCount))

	// Count raw sources
	rawCount := countFiles(rawDir)
	b.WriteString(fmt.Sprintf("Raw sources: %d\n", rawCount))

	// Recent log entries
	logPath := filepath.Join(wikiRoot, "log.md")
	if logContent, err := os.ReadFile(logPath); err == nil {
		lines := strings.Split(string(logContent), "\n")
		var recent []string
		for _, line := range lines {
			if strings.HasPrefix(line, "## [") {
				recent = append(recent, line)
			}
		}
		if len(recent) > 5 {
			recent = recent[len(recent)-5:]
		}
		if len(recent) > 0 {
			b.WriteString("\n**Recent activity:**\n")
			for _, r := range recent {
				b.WriteString(r + "\n")
			}
		}
	}

	return slashcmd.CommandResult{Output: b.String()}, nil
}

// wikiLog handles `/wiki log` — pure Go implementation.
func wikiLog(ctx slashcmd.Context) (slashcmd.CommandResult, error) {
	logPath := filepath.Join(getWikiRoot(ctx), "log.md")
	content, err := os.ReadFile(logPath)
	if err != nil {
		return slashcmd.CommandResult{
			Output: fmt.Sprintf("Could not read `%s` — wiki may not be initialized.", logPath),
		}, nil
	}
	return slashcmd.CommandResult{Output: string(content)}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────

// isWikiInitialized checks if the wiki has been initialized.
// It verifies both index.md and the wiki/ directory exist.
func isWikiInitialized(ctx slashcmd.Context) bool {
	root := getWikiRoot(ctx)
	if _, err := os.Stat(filepath.Join(root, "index.md")); os.IsNotExist(err) {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, "wiki")); os.IsNotExist(err) {
		return false
	}
	return true
}

// getWikiRoot returns the wiki root directory path.
func getWikiRoot(ctx slashcmd.Context) string {
	wd, err := os.Getwd()
	if err == nil {
		return filepath.Join(wd, wikiDir)
	}
	return wikiDir
}

// countFiles counts non-directory, non-dot files in a directory (non-recursive).
func countFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			count++
		}
	}
	return count
}
