---
type: entity
date: 2026-06-10
tags: [tools, lifecycle, hooks, middleware]
---

# Tool Lifecycle Hooks

> Before/After hooks and argument preparation for tool execution.

## Hooks

| Hook | Timing | Purpose |
|------|--------|---------|
| `Before` | Before Execute | Pre-processing, validation, logging |
| `After` | After Execute | Post-processing, auditing |
| `PrepareArguments` | Before Validate | Argument transformation |

## Execution Flow

```
PrepareArguments → Validate → Before hooks → Execute → After hooks
```

## Related

- [[tool-system]] — Hooks modify tool execution
- [[agent-core]] — LifecycleHooks is part of Agent Options