---
type: concept
date: 2026-06-10
tags: [architecture, decoupling, platform, application]
---

# Runtime/Application Interface

> The key architectural decoupling point between the Platform layer and the Application layer.

## Purpose

The `runtime.Application` interface is the single point of decoupling between:
- **Platform layer** (`internal/runtime/`) — Agent session lifecycle, domain-agnostic
- **Application layer** (`internal/agents/coding/`) — Domain-specific behavior (tools, prompts, commands)

This enables the "one kernel, three entrypoints" design principle.

## Interface

```go
type Application interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
    BuildPrompt(opts PromptBuildOptions, profile, goal string) string
    NewSessionExt() SessionExt
}
```

## BuildTools

Receives `ToolBuildOptions` containing:
- `Workspace`, `MaxOutputLen` — Sandbox configuration
- `BashOps`, `FileOps` — [[operations-abstract]] backends
- `ExtensionTools` — Tools from [[extension-system]]
- `AllowedTools`, `BlockedTools` — Filtering

## BuildPrompt

Receives `PromptBuildOptions` containing:
- `CustomPrompt`, `CWD` — Context
- `Tools` — Tool list for prompt generation
- `ContextFiles` — Additional context files
- `Skills` — [[skill-system]] entries

## SessionExt

Optional per-session extension providing:
- Profile management (get/switch)
- Goal management (get/set/clear)

## Related

- [[agent-core]] — The runtime creates Agent instances
- [[tool-system]] — Built via `BuildTools()`
- [[agent-loop]] — Loops are managed by runtime
- [[four-layer-architecture]] — This is the Platform↔App boundary