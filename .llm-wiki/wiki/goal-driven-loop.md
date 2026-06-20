---
type: concept
date: 2026-06-10
updated: 2026-06-21
tags: [goal, loop, evaluation, agent, llm-evaluator]
---

# Goal-Driven Loop

> A special agent execution mode where the LLM autonomously works toward a defined goal, with dual evaluation (LLM + keyword fallback).

## Behavior

When a goal is set (non-empty string):
- `maxTurns` limit is effectively **disabled** (set to 0 = unlimited)
- After each iteration, the LLM evaluates whether the goal is **completed**
- A follow-up reminder is automatically injected to keep the LLM on track
- Goal completion triggers `EventGoalCompleted` event

## Implementation

### Goal Storage
- Stored as `goal` string in [[agent-core]]
- `SetGoal(goal)` / `ClearGoal()` — Toggle goal mode
- Agent rebuild triggered on goal change (same as profile switch)

### Dual Evaluation System

The goal system uses two evaluation methods:

#### 1. Primary: LLM-Based Evaluator (`goal_evaluator.go`)

```go
func evaluateGoalCompletion(
    ctx context.Context,
    registry *providers.Registry,
    model ai.Model,
    assistantText string,
    goal string,
) (bool, string)
```

**Process**:
1. Build focused evaluator prompt with goal and assistant response
2. Call LLM with `MaxTokens: 256` for fast evaluation
3. Parse JSON response: `{"ok": true/false, "reason": "..."}`
4. Truncate assistant text to last 3000 runes to avoid excessive token usage

**Evaluator Prompt Rules**:
- If assistant is still actively working (reading files, running commands) → `{"ok": false}`
- If assistant has produced a final response addressing the objective → `{"ok": true}`
- If assistant explicitly says it is done → `{"ok": true}`
- When in doubt, consider whether a human would consider the task finished

#### 2. Fallback: Keyword-Based Detection (`goal.go`)

```go
func goalCompleted(responseText string) bool {
    // Conservative keyword matching for strong completion signals
    strongPhrases := []string{
        "goal has been achieved",
        "goal has been completed",
        "objective has been achieved",
        "目标已达成",
        "目标已实现",
        // ... more phrases
    }
}
```

**When Used**:
- LLM evaluator provider unavailable
- LLM stream error
- JSON parse failure
- Empty response

### Goal Logging

Debug logging to `/tmp/pi-goal-debug.log`:
- Agent creation with goal
- Goal-driven mode activation
- Evaluator LLM raw response
- Parsed evaluation result

## Use Case

Coding tasks where the agent needs to autonomously implement a feature, fix a bug, or explore a codebase without user intervention between steps.

## Events

| Event | Description |
|-------|-------------|
| `EventGoalCompleted` | Emitted when agent signals goal is fully achieved |

## Related

- [[agent-core]] — Goal is an Agent option; `LoopDetectSettings` controls loop detection
- [[agent-loop]] — Goal mode changes loop behavior (effectiveMaxTurns = 0)
- [[runtime-application-interface]] — SessionExt includes goal management
- [[tool-lifecycle-hooks]] — `SessionStartHook` receives goal in `SessionStartEvent`