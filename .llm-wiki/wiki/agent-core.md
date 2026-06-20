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
| ConfirmFunc | `ConfirmFunc` | Optional: dangerous tool execution confirmation (default: approve) |
| LoopDetectSettings | `LoopDetectSettings` | Loop detection config (default: enabled, threshold=5) |

## Key Methods

- **`Prompt()`** — Synchronous conversation, returns final assistant message
- **`PromptStream()`** — Async conversation via event channel (8 event types)
- **`CompactNow()`** — Manual trigger of [[context-compaction]]
- **`Steer()` / `FollowUp()`** — Queue messages for the agent loop
- **`SetGoal()` / `ClearGoal()`** — Manage [[goal-driven-loop]] mode

## Event System

The agent emits events via a pub/sub model. `PromptStream` subscribes and forwards to an event channel. There are now **13 stream event types**:

| Event Type | Description |
|------------|-------------|
| `text_delta` | Incremental text token from LLM |
| `turn_end` | Turn completed with assistant message |
| `tool_start` | Tool execution started |
| `tool_update` | Tool partial result update |
| `tool_end` | Tool execution completed |
| `tool_batch_start` | Tool batch started (parallel or sequential) |
| `done` | Conversation complete |
| `error` | Error occurred |
| `compacted` | Context compaction completed |
| `compaction_failed` | Context compaction failed |
| `confirmation_request` | Tool requires user confirmation (dangerous operation) |
| `confirmation_result` | User approved/denied the confirmation |
| `loop_detected` | Consecutive identical tool calls detected |
| `goal_completed` | Agent signals goal has been fully achieved |
| `micro_compacted` | MicroCompact cleared old tool results (zero LLM cost) |

## Confirmation Gate

Tools can implement `ToolWithConfirmation` to declare they need user approval before execution. The flow:

1. Tool's `RequiresConfirmation(args)` returns `(description, true)`
2. Agent emits `EventConfirmationRequest` with the description
3. `ConfirmFunc` (injected by entrypoint) asks the user for approval
4. If denied: tool is not executed, "user declined" is returned as tool result (IsError=false)
5. If `ConfirmFunc` is nil (serve/feishu): default approve

This is the primary safety mechanism for dangerous operations (e.g., `bash` commands, file overwrites).

## Loop Detection

The agent detects consecutive identical tool calls and injects soft reminders:

- **Setting**: `LoopDetectSettings{Enabled: true, Threshold: 5, ReminderTemplate: "..."}`
- **Behavior**: When the same tool is called with identical args N times in a row, emits `EventLoopDetected` and injects a follow-up reminder
- **Non-blocking**: Does not interrupt execution, just nudges the LLM to change approach
- **Per-prompt reset**: Loop detector resets at the start of each `Prompt`/`PromptStream` call

## Dependencies

- [[tool-system]] — Tools registered in the agent
- [[llm-provider-system]] — LLM backend via Registry
- [[session-persistence]] — Optional session storage
- [[context-compaction]] — Managed via SummarizeFunc
- [[agent-loop]] — The core execution loop
- [[external-tools]] — HTTP-registered tools dispatched during execution
- [[coding-application]] — The primary application layer driving the agent