---
type: entity
date: 2026-06-10
tags: [commands, slash, cli, control-plane]
---

# Slash Command Framework

> A registration-based command system for the agent's control plane.

## Architecture

- **Registry** (`internal/slashcmd/registry.go`) — Central command registration
- **Contexts** — Two context types for commands:
  - `SessionContext` — Session-level operations (model/profile/switch)
  - `AppContext` — App-level operations (session CRUD/profiles)

## Built-in Commands (14)

| Command | Purpose |
|---------|---------|
| `/help` | List commands |
| `/new` | New session |
| `/switch` | Switch session |
| `/sessions` | List sessions |
| `/session` | Session info |
| `/model` | Change model |
| `/models` | List models |
| `/tools` | List tools |
| `/profiles` | List profiles |
| `/profile` | Switch profile |
| `/compact` | Compact context |
| `/branch` | Create branch |
| `/goal` | Set/clear goal |
| `/context` | View context |
| `/clear` | Clear screen |

## Related

- [[runtime-application-interface]] — Commands operate through SessionExt
- [[agent-core]] — Commands affect agent state
- [[session-persistence]] — Session management commands