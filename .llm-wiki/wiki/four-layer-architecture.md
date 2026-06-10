---
type: concept
date: 2026-06-10
tags: [architecture, layers, design]
---

# Four-Layer Architecture

> The structural foundation of pi-go: strict unidirectional dependency between layers.

## Layer Diagram

```
┌──────────────────────────────────────┐
│  Entrypoints (组装与入口)             │
│  cmd/pi-agent  cmd/pi-feishu-bridge   │
├──────────────────────────────────────┤
│  Application (领域应用层，可插拔)      │
│  internal/agents/coding/              │
├──────────────────────────────────────┤
│  Platform (运行时平台层，领域无关)      │
│  internal/runtime/                    │
├──────────────────────────────────────┤
│  Core (核心层，零领域知识)             │
│  agent/  ai/  session/  compaction/   │
│  operations/  prompt/  skill/         │
│  extensions/  slashcmd/  tools/       │
└──────────────────────────────────────┘
```

## Dependency Rules

- **Core** → Does not depend on any upper layer
- **Platform** → Depends only on Core
- **Application** → Depends on Core + Platform via [[runtime-application-interface]]
- **Entrypoints** → Assembles all dependencies, injects Application instance

## Key Decoupling Point

The [[runtime-application-interface]] (`runtime.Application`) is the single boundary between Platform and Application layers. This enables:
- Multiple applications (coding, review, etc.) without modifying Platform
- Platform evolution without affecting Application
- "One kernel, three entrypoints" (CLI / Desktop / Feishu)

## Related Concepts

- [[runtime-application-interface]] — The Platform↔App boundary
- [[agent-core]] — Lives in Core layer
- [[tool-system]] — Lives in Core + assembled in Application
- [[llm-provider-system]] — Lives in Core