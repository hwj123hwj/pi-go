---
type: entity
date: 2026-06-10
tags: [skills, prompts, knowledge]
---

# Skill System

> Loads skill definitions from `.claude/skills/` directories into the agent's system prompt.

## How It Works

- Each skill is a directory with a `SKILL.md` file
- Loaded by the `internal/skill/` package
- Skills are injected into the system prompt during `BuildPrompt()`
- Supports Go-style conditional compilation tags for selective loading

## Skill Sources

| Path | Purpose |
|------|---------|
| `.claude/skills/` | Project-level skills |
| `~/.claude/skills/` | User-level skills |

## Related

- [[runtime-application-interface]] — `PromptBuildOptions` takes `[]skill.Skill`
- [[four-layer-architecture]] — Skill loading is in Core layer