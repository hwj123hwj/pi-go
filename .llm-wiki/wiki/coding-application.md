---
type: entity
date: 2026-06-14
tags: [coding-agent, application-layer, profile, prompts, tools]
---

# Coding Application

> The primary application layer of pi-go — a code-editing agent with tools, profiles, slash commands, and specialized prompts.

## Source: `internal/agents/coding/`

```
internal/agents/coding/
├── coding.go           ← Facade: exports RegisterCommands, BaseToolNames
├── application.go      ← CodingApplication: implements runtime.Application
├── session_ext.go      ← CodingSessionExt: per-session state (profile, goal)
├── cli/
│   └── interactive.go  ← Interactive CLI mode (conversation loop)
├── commands/
│   └── builtins.go     ← 13 slash commands (help, compact, model, profile, new, fork, ...)
├── deepv/
│   └── headers.go      ← DeepV-specific git metadata header provider
├── profile/
│   └── profile.go      ← Profile types & system prompts
├── prompt/
│   └── builder.go      ← System prompt builder
└── tools/
    ├── tools.go         ← Tool assembly (BuildList, ListOptions)
    └── file_mutation.go ← Per-file FIFO serialization for write safety
```

## Application Interface Implementation

`CodingApplication` implements `runtime.Application`:

```go
func (a *CodingApplication) BuildTools(opts runtime.ToolBuildOptions) []agent.Tool
func (a *CodingApplication) BuildPrompt(opts runtime.PromptBuildOptions, profile, goal string) string
func (a *CodingApplication) NewSessionExt() runtime.SessionExt
```

### BuildTools
- Assembles the 7 [[tool-system|built-in tools]]
- Applies tool filtering (AllowedTools / BlockedTools)
- Injects [[operations-abstract|Operations]] backends (local or SSH)
- Wraps file writes in per-file serialization queue (`file_mutation.go`)

### BuildPrompt
- Combines shared context ([[source-project-root|CLAUDE.md]], AGENTS.md)
- Injects skill content from [[skill-system]]
- Appends profile-specific prompt (coding vs review)
- Sets goal-driven instructions if applicable

## Profile System

Located in `internal/agents/coding/profile/profile.go`:

| Profile | Purpose | System Prompt Focus |
|---------|---------|-------------------|
| `coding` | General code editing | Standard agent instructions with tool usage |
| `review` | Code review mode | Review-first mindset, diff analysis |

Profiles are switchable at runtime via slash commands. Switching a profile triggers agent rebuild.

## Slash Commands

The coding application registers 13 commands (in `commands/builtins.go`):

| Command | Purpose |
|---------|---------|
| `/help` | List available commands |
| `/compact` | Manually trigger [[context-compaction]] |
| `/model` | Switch LLM model for current session |
| `/profile` | Switch coding/review profile |
| `/new` | Create a new session |
| `/fork` | Fork session at current point |
| `/sessions` | List all sessions |
| `/clear` | Clear screen |
| `/info` | Session info |
| `/goal` | Set/clear [[goal-driven-loop\|goal]] |
| `/web` | Web search (when available) |
| `/export` | Export session |
| `/feishu` | Feishu integration commands |

## Session Extension

`CodingSessionExt` (in `session_ext.go`) manages per-session state:
- **Profile** — Current profile (coding/review), gettable and switchable
- **Goal** — Current goal string, gettable and settable
- **Rebuild** — Rebuilds the agent on profile/goal changes

## Interactive CLI

`internal/agents/coding/cli/interactive.go` provides:
- Conversation loop with slash command dispatch
- Terminal rendering via [[tui-presenter]]
- Session management (new, switch, fork)
- Timeouts and error handling

## DeepV Integration

`internal/agents/coding/deepv/headers.go` provides git metadata headers for DeepV-specific features (branch info, commit context).

## Related

- [[runtime-application-interface]] — CodingApplication implements this interface
- [[tool-system]] — Tools are assembled in BuildTools
- [[slash-command-framework]] — Commands registered via RegisterCommands
- [[tui-presenter]] — CLI user interface
- [[profile-system]] — Coding vs Review profiles
