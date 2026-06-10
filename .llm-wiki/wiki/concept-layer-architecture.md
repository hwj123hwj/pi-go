---
type: concept
date: 2026-06-10
tags: [pi-go, concept, architecture, layers]
source: "source-project-root.md"
---

# Layer Architecture

Strict four-layer hierarchy with unidirectional dependencies.

## The Four Layers

```
Entrypoints (cmd/pi-agent, cmd/pi-feishu-bridge) -- assembly & entry
    |
Application (internal/agents/coding/) -- pluggable domain layer
    |
Platform (internal/runtime/) -- agent session lifecycle, Application interface
    |
Core (internal/agent/, internal/ai/, internal/session/, ...) -- zero domain knowledge
```

## Dependency Rules

1. Core depends on nothing above it
2. Platform depends only on Core
3. Application implements runtime.Application interface
4. Entrypoints wire everything together

## Key Interfaces

| Layer | Interface | File |
|-------|-----------|------|
| Platform | runtime.Application | internal/runtime/application.go |
| Platform | runtime.SessionExt | internal/runtime/application.go |
| Core | agent.Tool | internal/agent/tool.go |
| Core | providers.Provider | internal/ai/providers/ |
| Core | operations.Operations | internal/operations/interface.go |

## Motivations

- Pluggable applications: swap coding-agent by implementing runtime.Application
- Testability: Core independently testable, no domain knowledge
- Minimal dependencies per layer

## [[wikilinks]]

- App Layer
- Agent Loop
- Extension System
