---
type: concept
date: 2026-06-22
tags: [ai-agents, coding-conventions, claude-code, codex, development-workflow]
related: [[coding-application]], [[skill-system]]
---

# Agent Guidance System

> pi-go provides structured guidance to AI coding agents (Claude Code, Codex) through dedicated configuration files. These files establish coding conventions, project architecture awareness, and behavioral guidelines.

## Files

### `.claude/CLAUDE.md`
Guidance for **Claude Code** (claude.ai/code). Identical content to AGENTS.md, tailored for Claude Code's execution model.

### `AGENTS.md`
Guidance for **Codex** (codex.ai/code). Same behavioral conventions, same project architecture reference.

### `.claude/settings.json`
Configuration for Claude Code skills and plugins:
```json
{
  "skills": ["docs-maintainer", "research", "handoff"],
  "enabledPlugins": { "codex@openai-codex": true }
}
```

### `.agents/skills/`
Seven agent skills available for context injection:
- **docs-maintainer** — Documentation maintenance workflows
- **golang-patterns** — Go idioms and patterns
- **golang-testing** — Go testing conventions
- **grill-me** — Code review / challenge mode
- **grill-with-docs** — Documentation-aware review
- **handoff** — Session handoff protocols
- **research** — Research and analysis workflows

## Behavioral Conventions (from CLAUDE.md / AGENTS.md)

### Think Before Code
- Don't assume. Ask when confused.
- Propose simpler alternatives when they exist. Push back when appropriate.

### Simplicity First
- Minimum code to solve the problem. No features beyond requirements.
- No abstractions for single calls, no error handling for impossible scenarios.
- Self-check: "Would a senior engineer consider this over-engineered?"

### Precise Changes
- Only touch what must change. Don't optimize adjacent code.
- Match existing style, even if yours is better.
- Clean up orphaned code from your changes (imports/variables/functions), but don't delete pre-existing dead code.

### Goal-Driven Execution
- Convert tasks to verifiable goals: "add validation" → "write test cases first, then make them pass"
- Multi-step tasks: plan first, each step with verification criteria.

## Project Architecture Reference

Both files reference the same four-layer architecture:
```
cmd/pi-agent  cmd/pi-feishu-bridge     ← Entrypoints
internal/agents/coding/                ← Application layer
internal/runtime/                      ← Platform layer
internal/agent/ ai/ session/ ...       ← Core layer
```

## Core Interfaces Referenced

| Interface | File | Purpose |
|-----------|------|---------|
| `agent.Tool` | `internal/agent/tool.go` | Tool system (optional: ToolWithMode, ConcurrencySafeChecker, ToolWithPrepareArguments) |
| `providers.Provider` | `internal/ai/providers/interface.go` | LLM Provider registration (Name + Stream + StreamSimple) |
| `runtime.Application` | `internal/runtime/application.go` | Platform↔App decoupling: BuildTools() + BuildPrompt() + NewSessionExt() |
| `operations.Operations` | `internal/operations/interface.go` | Local/SSH execution backend switching |

## Related

- [[skill-system]] — Skills loaded from `.claude/skills/` directory
- [[coding-application]] — Primary consumer of agent guidance
- [docs/CONTRIBUTING.md](../../docs/CONTRIBUTING.md) — Full contributor guide with code standards
