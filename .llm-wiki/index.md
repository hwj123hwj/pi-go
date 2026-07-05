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
- [[source-project-root-v14]] — Re-ingest v14: Music player robustness fixes (stale URL cache retry, lyrics JSON injection, cache cleanup, audio element reload, error recovery) (2026-06-25)
- [[source-project-root-v15]] — Re-ingest v15: Music deep-audit round 2 (FD leak fix, 403 stale detection, retry exhaustion, multi-try fallback, React audio lifecycle race) (2026-06-25)
- [[source-project-root-v16]] — Re-ingest v16: Music personalization (preference store, listening history, fixed-size prompt injection, music_history tool) (2026-06-26)
- [[source-project-root-v17]] — Re-ingest v17: Unified user profile (cross-agent second brain, OpenViking-inspired, all agents share fixed-size profile summary) (2026-06-26)
- [[source-project-root-v18]] — Re-ingest v18: OpenViking deep-absorption (KB L1 overview mode, hotness-based memory eviction with frequency×recency) (2026-06-26)
- [[source-project-root-v19]] — Re-ingest v19: KB vector search (SiliconFlow bge-m3, hybrid keyword+vector retrieval, local JSON vector store) (2026-06-26)
- [[source-project-root-v20]] — Re-ingest v20: Session memory extraction (OpenViking ExtractLoop adaptation, LLM-based user fact extraction, async non-blocking) (2026-06-26)
- [[source-project-root-v21]] — Re-ingest v21: Tool output auto-synopsis (context window protection, After-hook replaces large outputs with structural synopsis) (2026-06-26)
- [[source-project-root-v22]] — Re-ingest v22: Code review fixes for v17–v21 (UTF-8 safety, mutex copy, error logging, import double-counting) (2026-06-27)
- [[source-project-root-v23]] — Re-ingest v23: Code review round 2 — stale vectors, batch eviction, phantom results, double-synopsis (2026-06-27)
- [[source-project-root-v24]] — Re-ingest v24: Code review round 3 — stale removal persistence, scanner buffer limit (2026-06-27)
- [[source-project-root-v25]] — Re-ingest v25: Desktop profile panel — user profile UI adaptation (2026-06-27)
- [[source-project-root-v26]] — Re-ingest v26: Desktop UI fixes (bottom terminal, sidebar toggle, profile error), KB absolute paths, cmd/pi-music removal (2026-06-27)
- [[source-project-root-v27]] — Re-ingest v27: Mobile UX overhaul round 1 (PromptBar compact, MusicBar full-width, audio proxy HTML rejection, Capacitor details) (2026-06-27)
- [[source-project-root-v28]] — Re-ingest v28: Mobile UX overhaul round 2 (chat transcript density, tool cards touch targets, Markdown mobile rendering, keyboard-adaptive dvh, inline music card compact) (2026-06-27)
- [[source-project-root-v29]] — Re-ingest v29: Mobile UX overhaul round 3 (Diff preview mobile, modal/dialog full-screen, code block copy + collapse, tool body compact, momentum scroll, session list touch) (2026-06-27)
- [[source-project-root-v30]] — Re-ingest v30: Mobile right sidebar CSS fix (file browser now accessible), music bar close button, file panel touch-friendly (2026-06-27)
- [[source-project-root-v31]] — Re-ingest v31: Mobile right sidebar close button (back arrow), PromptBar stop button circular icon, file panel touch details (2026-06-27)
- [[source-project-root-v32]] — Re-ingest v32: Mobile toolbar compact density toggle, empty state (hide folder picker/model selector), keyboard auto-dismiss on scroll (2026-06-27)
- [[source-project-root-v33]] — Code review: fix duplicate CSS (toolbar-density-mobile, rsidebar-content), GlobalMusicBar audio src cleanup, remove unused playMusic var (2026-06-27)
- [[source-project-root-v34]] — Code review round 2: fix file tree not hidden on mobile (wrong CSS class), ChatPane onTouchStart stealing focus from buttons (2026-06-27)
- [[source-project-root-v35]] — Code review round 3: fix sidebar backdrop double-dimming on mobile, btn-stop mobile CSS missing !important (2026-06-27)
- [[source-project-root-v36]] — Code review round 4: fix rsidebar-content overflow conflict (hidden vs auto), CodeBlock clipboard unhandled rejection (2026-06-27)
- [[source-project-root-v37]] — Mobile toolbar declutter: hide density toggle, hide sidebar+bottom toggles, keep only right-panel toggle, justify-content space-between layout (2026-06-27)
- [[source-project-root-v38]] — In-app self-update: custom ApkUpdaterPlugin (native APK download + install intent), MobileUpdateDialog UI, version 0.6.0 release (2026-06-27)
- [[source-project-root-v39]] — Re-ingest v39: manual check-update button, model selector restored, music bar transform fix, version management lesson (2026-06-28)
- [[source-project-root-v40]] — Voice input (ASR): TeleSpeechASR via SiliconFlow, MediaRecorder API, server proxy, /asr/transcribe endpoint (2026-06-28)

## Entities
- [[agent-core]] — Agent state machine and execution engine
- [[tool-system]] — 8 built-in tools + optional interfaces + external tools
- [[web-fetch-tool]] — 8th built-in tool: URL→markdown with SSRF protection
- [[music-agent]] — Bilibili-primary + NetEase recommendation music application layer (7 tools, preference store for personalization)
- [[kb-agent]] — Knowledge base agent: search, browse, read, write, maintain (5 tools + desktop KB panel + hybrid vector search v19 + L1 overview v18)
- [[llm-provider-system]] — Anthropic/OpenAI providers (Mock removed v7, DeepV removed v6)
- [[session-persistence]] — JSONL append-only with tree branching
- [[session-manager]] — Session CRUD, forking, listing, metadata (sessionmgr package)
- [[slash-command-framework]] — 14 built-in slash commands
- [[tool-lifecycle-hooks]] — 9-step execution flow, Before/After hooks, confirmation gate, session observer hooks
- [[extension-system]] — Plugin-style tools/commands/hooks
- [[skill-system]] — Markdown skill loading from `.claude/skills/`
- [[desktop-app]] — Electron + React GUI client (global music player, workspace layout, KB panel, profile panel v25, path clicking, v10 race condition fixes, mobile self-update v38, version management v39, voice input/ASR v40)
- [[feishu-integration]] — Lark/Feishu bot bridge
- [[server-websocket]] — HTTP REST + SSE + WebSocket server (protocol, file endpoints, KB endpoints, profile endpoints v25, gateway models, ASR endpoint v40)
- [[config-system]] — Environment-driven configuration (PI_GO_* env priority, LoadDotEnv no-override, provider required v7, KB embedding config v19, ASR config v40)
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
- [[personal-assistant-roadmap]] — Evolution to general personal assistant, memory layer design (P0–P1.5 implemented v14–v25)
- [[competitive-analysis]] — DeepV feature gap analysis, 30+ features prioritized (NEW)
- [[unified-profile]] — Cross-agent persistent user profile ("condensed second brain"), hotness eviction, fixed-size summary injection (NEW v17)
- [[kb-vector-search]] — Hybrid keyword + vector retrieval (SiliconFlow bge-m3, local JSON vector store) (NEW v19)
- [[session-memory-extraction]] — Async LLM-based user fact extraction after each turn (OpenViking ExtractLoop adaptation) (NEW v20)
- [[tool-output-synopsis]] — After-hook that replaces large tool outputs with deterministic structural synopsis (NEW v21)
- [[asr-voice-input]] — Speech-to-text pipeline: MediaRecorder → server proxy → SiliconFlow TeleSpeechASR (NEW v40)

- [[react-native-mobile-app]] — Mobile client rebuilt from Capacitor to React Native + Expo (RN best practices: Hermes mmap, R8 shrinking, React.memo, FlatList optimization, Zustand atomic selectors) (NEW v41)
- [[loop-scheduler]] — /loop watchdog recurring loop (goroutine + time.Ticker) + TASK.md handoff context persistence (NEW, absorbed from hwjcode)
- [[model-registry-feishu-fix]] — Config-driven model registry (JSON-configurable, 13 built-in models) + Feishu multi-chat sender race fix (NEW)

## Synthesis
<!-- Cross-cutting analysis pages will be listed here -->
