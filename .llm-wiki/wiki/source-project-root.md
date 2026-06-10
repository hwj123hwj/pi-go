---
type: source
source_path: "."
date: 2026-06-10
tags: [pi-go, source, project-root, architecture]
---

# Source: Project Root (`.`)

> Ingested from the project root directory.

## Key Takeaways

- Pi-Go is a **general-purpose Agent framework** in Go 1.24+, not just a coding agent
- Four-layer architecture: Entrypoints -> Application -> Platform -> Core (strict top-down)
- Zero domain knowledge in Core layer
- 14+ built-in slash commands, 7 built-in tools
- Supports 4 LLM providers: Anthropic, OpenAI, DeepV, Mock
- Feishu bridge is a standalone service connecting Agent to Feishu group chat
- Desktop client: Electron + React + TypeScript (under desktop/)

## Important Entities & Concepts

| Name | Type | Summary |
|------|------|---------|
| App Layer | entity | Thin assembly: providers, sessions, extensions, ops |
| Feishu Bridge | entity | Standalone service: WS gateway, SSE streaming, CardKit |
| Agent Loop | entity | Dual-loop: outer=follow-up, inner=tool call |
| AI Providers | entity | Registration pattern: Name + Stream + StreamSimple |
| Tool System | entity | Tool interface + optional extensions |
| Extension System | entity | Tools + commands + event hooks |
| Layer Architecture | concept | 4-layer strict: Core -> Platform -> Application -> Entrypoints |
| Goal-Driven Loop | concept | Goal set -> maxTurns=0, LLM evaluates completion |

## Notable Claims

- Core depends on nothing above it -- verified in code
- App layer is thin assembly -- verified: only wires deps
- runAgentLoop() shared between RunLoop and PromptStream -- verified
- Mock provider always registered
- Feishu gateway uses WebSocket long connection, not webhook

## Contradictions

None with existing wiki content.
