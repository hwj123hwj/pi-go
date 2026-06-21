---
type: concept
date: 2026-06-22
tags: [architecture, layers, design, decision-framework]
related: [[runtime-application-interface]], [[agent-core]], [[tool-system]], [[llm-provider-system]]
---

# Four-Layer Architecture

> The structural foundation of pi-go: strict unidirectional dependency between layers.

## Layer Diagram

```
┌─────────────────────────────────────────────┐
│  Entrypoints (组装与入口)                    │
│  cmd/pi-agent  cmd/pi-feishu-bridge  cmd/pi-music  │
├─────────────────────────────────────────────┤
│  Application (领域应用层，可插拔)             │
│  agents/coding/ — 工具集、提示、命令、Profile │
│  agents/music/  — 音乐助手应用               │
│  agents/kb/     — 知识库检索应用             │
├─────────────────────────────────────────────┤
│  Platform (运行时平台层，领域无关)            │
│  runtime/ — AgentSession 生命周期、Application 接口 │
├─────────────────────────────────────────────┤
│  Core (核心层，零领域知识)                    │
│  agent/  ai/  session/  compaction/          │
│  operations/  prompt/  skill/                │
│  extensions/  slashcmd/                      │
└─────────────────────────────────────────────┘
```

**Special case**: `internal/music/` is a domain-specific leaf library (music sources + proxy + cache), only imported by music agent and entrypoint. It doesn't belong to the standard four layers.

## Dependency Rules

- **Core** → Does not depend on any upper layer
- **Platform** → Depends only on Core
- **Application** → Depends on Core + Platform via [[runtime-application-interface]]
- **Entrypoints** → Assembles all dependencies, injects Application instance
- **Domain Infrastructure** (e.g. `internal/music/`) → Leaf library of Application layer, zero upper dependencies

## Key Decoupling Point

The [[runtime-application-interface]] (`runtime.Application`) is the single boundary between Platform and Application layers. This enables:
- Multiple applications (coding, music, etc.) without modifying Platform
- Platform evolution without affecting Application
- "One kernel, three entrypoints" (CLI / Desktop / Feishu)

## Skills vs Application Decision Framework

When expanding pi-go with new agent capabilities, the decision of **Skills (vertical)** vs **independent Application (horizontal)** follows a 3-layer judgment:

### Decision Flow

```
Did the execution loop change? (event-driven / multi-tenant / DAG / autonomous)
  │
  ├─ Yes → Cannot rely on skills alone. May need new Application + new entrypoint + new scheduler
  │
  └─ No → Is the default behavior contract severely conflicting?
            │
            ├─ No (tool subset / prompt variation / role switch)
            │    → Vertical: Skills / Modes / Tool Filtering
            │
            └─ Yes (completely different tools / workspace / behavior)
                 │
                 ├─ Is this a secondary feature?
                 │    ├─ Yes → Skill / extension attachment
                 │    └─ No (first-class citizen product form)
                 │         → Independent Application (shared Platform)
                 │         Example: Music agent ← validated this path
```

### Key Insight

> **Skills change "who the agent is"; Application changes "how the agent runs".** But if "who" has changed beyond recognition (completely different tools, workspace, behavior) AND it's a first-class citizen → split Application. Cost is low because Platform is shared.

### Concrete Examples

| Scenario | Loop Changed? | Contract Conflict? | First-Class? | Decision |
|----------|--------------|-------------------|-------------|----------|
| Code Review mode | No | Low | No | Skill + tool filtering |
| Music Agent | No | High | Yes | **Independent Application** ✅ |
| KB Agent | No | High | Yes | **Independent Application** ✅ |
| Feishu Bot | Yes (event-driven) | Yes | Yes | New Application + webhook entrypoint |
| Browser Agent (first-class) | No | High | Yes | Independent Application |
| Browser (secondary feature) | No | Medium | No | Skill/extension |

## Related Concepts

- [[runtime-application-interface]] — The Platform↔App boundary
- [[agent-core]] — Lives in Core layer
- [[tool-system]] — Lives in Core + assembled in Application
- [[llm-provider-system]] — Lives in Core
- [[personal-assistant-roadmap]] — Extends Application with memory and self-description
- [[music-agent]] — First non-coding Application, validates the split decision
- [[kb-agent]] — Third Application, simplest (read-only retrieval), validates low-cost of adding new apps
