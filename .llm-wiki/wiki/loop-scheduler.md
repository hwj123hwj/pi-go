# Loop Scheduler & Task Handoff

> Source: `internal/scheduler/`, `internal/handoff/`
> Inspired by: hwjcode `/loop` watchdog + TASK.md handoff design

## Overview

Two features absorbed from the hwjcode (deepvcode) project:

1. **`/loop` — Watchdog recurring loop**: Schedule a recurring prompt that fires at a fixed interval
2. **TASK.md Handoff**: Persistent task context that survives session restarts

## /loop Command

### Architecture

```
User: /loop 5m run go test ./...
         │
         ▼
slashcmd.Handler (commands/loop.go)
         │
         ▼
scheduler.LoopManager.Start(sessionID, prompt, interval, ttl)
         │
         ▼
goroutine + time.Ticker ─── fires every interval ───▶ TriggerResolver
                                                         │
                                                         ▼
                                                  server.injectLoopPrompt()
                                                         │
                                                         ▼
                                                  sess.Prompt(ctx, loopPrompt)
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Go goroutine + `time.Ticker` | Lightweight, no external deps (vs hwjcode's JS setInterval) |
| Per-session loops (max 1) | Matches hwjcode semantics; prevents prompt flooding |
| TriggerResolver pattern | Decouples scheduler from server — scheduler doesn't import `server` package |
| Auto-expiry (default 3 days) | Prevents zombie loops from running forever |
| Minimum interval 1 minute | Prevents API rate limiting |

### Usage

```
/loop 5m run go test ./...              # Start: runs "run go test ./..." every 5 minutes
/loop 1h check CI status --expires 8h   # With 8-hour lifetime
/loop                                    # Show current loop status
/loop clear                              # Stop the active loop
```

### Files

| File | Role |
|------|------|
| `internal/scheduler/loop.go` | LoopManager — goroutine lifecycle, tick firing |
| `internal/scheduler/duration.go` | Duration string parsing ("5m" → 5*time.Minute) |
| `internal/agents/coding/commands/loop.go` | `/loop` slash command handler |
| `internal/server/server.go` | TriggerResolver wiring (loop → session prompt injection) |

## TASK.md Handoff

### Architecture

```
Agent working on task...
         │
         ▼
/task save                    /task <new goal>
         │                         │
         ▼                         ▼
handoff.Save(workspace, doc)  handoff.NewTaskDocument(goal)
         │
         ▼
.easycode/TASK.md  ←─── persists to disk

─────── session restart / new session ───────

Agent builds system prompt
         │
         ▼
prompt/builder.go calls handoff.LoadAsPrompt(cwd)
         │
         ▼
TASK.md content injected into system prompt
         │
         ▼
Agent resumes task with full context
```

### Design Philosophy

> "Documents are the most reliable context transfer mechanism." — hwjcode

Unlike in-memory session state, a file survives:
- Process crashes / restarts
- Session switching
- Server redeployment
- Power outages

### TASK.md Structure

```markdown
# 📋 Task Handoff

**Goal:** Implement /loop feature
**State:** in-progress
**Created:** 2026-07-05 08:00:00
**Updated:** 2026-07-05 09:00:00

## ✅ Progress
- Created scheduler package
- Wrote tests

## ➡️ Next Steps
- [ ] Wire into server
- [ ] Add slash command

## 🚫 Blockers
- Waiting on API review

## 📁 Files Changed
- `internal/scheduler/loop.go`

## 📝 Notes
Need to handle concurrent access carefully.
```

### Usage

```
/task Implement user auth             # Create new task with goal
/task show                            # Display current task
/task save                            # Save current goal as handoff
/task done                            # Mark task as completed
/task clear                           # Remove task file
```

### Auto-Loading

TASK.md is **automatically loaded** into the system prompt on every agent build
(see `prompt/builder.go`). The agent sees the full handoff context without any
explicit command — it just picks up where the previous session left off.

### Files

| File | Role |
|------|------|
| `internal/handoff/task.go` | TaskDocument struct, Save/Load/Render/Clear |
| `internal/agents/coding/commands/task.go` | `/task` slash command handler |
| `internal/agents/coding/prompt/builder.go` | Auto-injection of TASK.md into system prompt |
