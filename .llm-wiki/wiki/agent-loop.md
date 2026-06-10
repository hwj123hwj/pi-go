---
type: concept
date: 2026-06-10
tags: [loop, agent-execution, dual-loop]
---

# Agent Dual-Loop

> The core execution pattern: an outer follow-up loop wrapping an inner tool-call loop.

## Structure

```
Outer Loop (follow-up)
  ├── Call LLM with messages
  ├── Inner Loop (tool calls)
  │   ├── Parse tool calls from response
  │   ├── Partition into safe batches
  │   ├── Execute each batch
  │   └── Send results back to LLM
  ├── Check stop reason
  │   ├── "end_turn" → output result, wait for user
  │   ├── "tool_use" → continue inner loop
  │   └── Goal-driven: evaluate completion
  └── If follow-up received → continue outer loop
```

## Implementation

- `RunLoop()` — Shared core for both `Prompt` and `PromptStream`
- `runAgentLoop()` — The actual loop logic, parameterized by `consumeStreamFunc`
- Callback pattern allows different streaming behaviors:
  - `Prompt` → collects full response
  - `PromptStream` → emits text delta events

## Goal-Driven Mode

When a [[goal-driven-loop]] is set:
- maxTurns is disabled (effectively infinite)
- After each iteration, LLM evaluates goal completion
- Automatic follow-up reminder is injected

## Related

- [[agent-core]] — The loop is the execution engine of the Agent
- [[tool-system]] — Tool execution happens inside the inner loop
- [[goal-driven-loop]] — Special mode for goal-directed execution