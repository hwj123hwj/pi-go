---
type: concept
date: 2026-06-10
tags: [pi-go, concept, agent, loop, dual-loop]
source: "source-project-root.md"
---

# Agent Loop

Dual-loop execution engine in internal/agent/loop.go.

## Structure

Outer Loop (follow-up) -> Inner Loop (tool calls + LLM)
- LLM Response: text + tool_calls -> Execute tools -> continue inner
- text (no tools) -> check follow-up -> continue outer OR done
- error/aborted -> done
- Goal-driven: if goal active & not done, inject reminder

## Shared Core: runAgentLoop()

Both RunLoop (interactive) and PromptStream (streaming API) share runAgentLoop().
Difference is consumeStreamFunc callback:
- RunLoop: reads EventDone / EventError only
- PromptStream: reads all events (text_delta, tool_call, etc.)

## Turn Processing (processTurn())

Each turn: append pending -> persist to session -> check compaction -> call LLM ->
handle message -> if tool calls: partition -> execute -> append results -> continue ->
if follow-up: return to outer -> if goal active: evaluate completion

## Tool Partitioning

- Parallel batches: consecutive ConcurrencySafeChecker tools in goroutines
- Sequential batches: each unsafe tool gets its own batch
- Panic recovery in both modes

## Goal-Driven Mode

When goal is active:
- effectiveMaxTurns = 0 (unlimited)
- LLM evaluates completion each turn
- Auto-injects follow-up reminder if not done

## [[wikilinks]]

- Tool System
- Goal-Driven Loop
- Context Compaction
