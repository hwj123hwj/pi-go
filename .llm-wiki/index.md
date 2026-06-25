# LLM Wiki Index

> Auto-maintained by Easy Code. Do not edit manually.

## Sources
- [[source-project-root]] — Full ingest of project root (.)
- [[source-project-root-v2]] — Comprehensive re-ingest with new findings (2026-06-21)
- [[source-project-root-v3]] — Re-ingest v3: decisions, deployment, desktop internals, agent guidance (2026-06-22)
- [[source-project-root-v4]] — Re-ingest v4: tool execution internals, session management, WebSocket protocol, desktop path clicking (2026-06-22)
- [[source-project-root-v5]] — Re-ingest v5: bilibili-primary music, global player, workspace layout overhaul (2026-06-23)
- [[source-project-root-v6]] — Re-ingest v6: deepv removed, config overhaul, PI_GO_API_KEY, LoadDotEnv priority fix (2026-06-23)
- [[source-project-root-v7]] — Re-ingest v7: mock removed, provider required, test infrastructure cleanup (2026-06-24)
- [[source-project-root-v8]] — Re-ingest v8: KB panel crash fix, route registration, null tags guard (2026-06-25)
- [[source-project-root-v9]] — Re-ingest v9: KB panel hardening (CSS var fix, path traversal fix, debounce, sorting) (2026-06-25)
- [[source-project-root-v10]] — Re-ingest v10: Desktop race condition & memory leak fixes (React unmount guards, WS backoff, empty dir fallback) (2026-06-25)
- [[source-project-root-v11]] — Re-ingest v11: Desktop i18n gaps, React non-reactive read, sidebar menu dismiss (2026-06-25)
- [[source-project-root-v12]] — Re-ingest v12: Desktop chat smart scroll, KB tag view loading state, tag back button tooltip (2026-06-25)
- [[source-project-root-v13]] — Re-ingest v13: Workspace inline file editor, PUT /workspace/write-file endpoint (2026-06-25)

## Entities
- [[agent-core]] — Agent state machine and execution engine
- [[tool-system]] — 8 built-in tools + optional interfaces + external tools
- [[web-fetch-tool]] — 8th built-in tool: URL→markdown with SSRF protection
- [[music-agent]] — Bilibili-primary + NetEase recommendation music application layer (6 tools)
- [[kb-agent]] — Knowledge base agent: search, browse, read, write, maintain (5 tools + desktop KB panel + v9 hardening)
- [[llm-provider-system]] — Anthropic/OpenAI providers (Mock removed v7, DeepV removed v6)
- [[session-persistence]] — JSONL append-only with tree branching
- [[session-manager]] — Session CRUD, forking, listing, metadata (sessionmgr package)
- [[slash-command-framework]] — 14 built-in slash commands
- [[tool-lifecycle-hooks]] — 9-step execution flow, Before/After hooks, confirmation gate, session observer hooks
- [[extension-system]] — Plugin-style tools/commands/hooks
- [[skill-system]] — Markdown skill loading from `.claude/skills/`
- [[desktop-app]] — Electron + React GUI client (global music player, workspace layout, KB panel, path clicking, v10 race condition fixes)
- [[feishu-integration]] — Lark/Feishu bot bridge
- [[server-websocket]] — HTTP REST + SSE + WebSocket server (protocol, file endpoints, KB endpoints, gateway models)
- [[config-system]] — Environment-driven configuration (PI_GO_* env priority, LoadDotEnv no-override, provider required v7)
- [[coding-application]] — Coding agent (primary application layer)
- [[tui-presenter]] — Terminal UI rendering system
- [[external-tools]] — HTTP callback tool registration
- [[web-embed]] — Embedded SPA static file serving
- [[ai-transform-retry]] — Message transform, retry, cost, model registry
- [[deployment-infrastructure]] — GitHub Actions CI/CD, Ubuntu systemd deployment (NEW)
- [[agent-guidance-system]] — CLAUDE.md/AGENTS.md coding conventions for AI agents (NEW)

## Concepts
- [[micro-compact]] — LLM-free compaction: clear old tool results at 60% threshold
- [[four-layer-architecture]] — Entrypoints → Application → Platform → Core (updated: skills-vs-app decision framework)
- [[runtime-application-interface]] — The Platform↔App decoupling boundary
- [[agent-loop]] — Outer follow-up + inner tool-call dual loop
- [[context-compaction]] — LLM-driven conversation summarization (updated: cross-framework comparison)
- [[goal-driven-loop]] — Autonomous goal-directed agent execution with LLM evaluator
- [[operations-abstract]] — Local/SSH execution backend switching
- [[personal-assistant-roadmap]] — Evolution to general personal assistant, memory layer design (NEW)
- [[competitive-analysis]] — DeepV feature gap analysis, 30+ features prioritized (NEW)

## Synthesis
<!-- Cross-cutting analysis pages will be listed here -->
