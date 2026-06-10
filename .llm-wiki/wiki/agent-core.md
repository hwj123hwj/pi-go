---
type: entity
date: 2026-06-10
tags: [agent, core, state-machine, loop]
---

# Agent Core

> The central state machine and execution engine of pi-go.

## State Machine

The [[agent-core]] has 4 states:

| State | Description |
|-------|-------------|
| `StateIdle` | Ready to accept new prompts |
| `StateRunning` | Currently executing a conversation |
| `StateWaiting` | Awaiting external input (e.g., user follow-up) |
| `StateError` | Recovered from an error |

## Agent Options

The `Agent` is configured via `Options` struct:

| Field | Type | Purpose |
|-------|------|---------|
| Model | `ai.Model` | LLM model selection |
| Registry | `*providers.Registry` | Provider lookup |
| System | `string` | System prompt |
| Tools | `[]Tool` | Available tools |
| MaxTurns | `int` | Max conversation turns |
| Goal | `string` | Activates [[goal-driven-loop]] |
| Session | `*session.Session` | Persistence backend |
| CompactionSettings | `compaction.Settings` | [[context-compaction]] config |
| SummarizeFunc | `SummarizeFunc` | LLM summarizer for compaction |
| LifecycleHooks | `LifecycleHooks` | [[tool-lifecycle-hooks]] |

## Key Methods

- **`Prompt()`** — Synchronous conversation, returns final assistant message
- **`PromptStream()`** — Async conversation via event channel (8 event types)
- **`CompactNow()`** — Manual trigger of [[context-compaction]]
- **`Steer()` / `FollowUp()`** — Queue messages for the agent loop
- **`SetGoal()` / `ClearGoal()`** — Manage [[goal-driven-loop]] mode

## Event System

The agent emits events via a pub/sub model. `PromptStream` subscribes and forwards to an event channel with 8 event types: `text_delta`, `turn_end`, `tool_start`, `tool_update`, `tool_end`, `done`, `error`, `compacted`.

## Dependencies

- [[tool-system]] — Tools registered in the agent
- [[llm-provider-system]] — LLM backend via Registry
- [[session-persistence]] — Optional session storage
- [[context-compaction]] — Managed via SummarizeFunc
- [[agent-loop]] — The core execution loop