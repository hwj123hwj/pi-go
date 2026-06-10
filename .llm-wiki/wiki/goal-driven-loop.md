---
type: concept
date: 2026-06-10
tags: [goal, loop, evaluation, agent]
---

# Goal-Driven Loop

> A special agent execution mode where the LLM autonomously works toward a defined goal.

## Behavior

When a goal is set (non-empty string):
- `maxTurns` limit is effectively **disabled**
- After each iteration, the LLM evaluates whether the goal is **completed**
- A follow-up reminder is automatically injected to keep the LLM on track

## Implementation

- Stored as `goal` string in [[agent-core]]
- `SetGoal(goal)` / `ClearGoal()` — Toggle goal mode
- Evaluated by [[goal-evaluator]]
- Logged via `goalLog()` for debugging

## Use Case

Coding tasks where the agent needs to autonomously implement a feature, fix a bug, or explore a codebase without user intervention between steps.

## Related

- [[agent-core]] — Goal is an Agent option
- [[agent-loop]] — Goal mode changes loop behavior
- [[runtime-application-interface]] — SessionExt includes goal management