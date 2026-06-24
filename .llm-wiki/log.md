# LLM Wiki Log

> Chronological record of wiki operations.

## [2026-06-25] ingest | Project Root (.) — v11: Desktop i18n gaps, React subscription bug, menu dismiss
- **Created** `source-project-root-v11.md` — Source summary: hardcoded English strings (4 items), non-reactive store read in ChatPane, sidebar new-session menu stuck open
- **Updated** `index.md` — Added source-project-root-v11

## [2026-06-25] ingest | Project Root (.) — v10: Desktop race condition & memory leak fixes
- **Created** `source-project-root-v10.md` — Source summary: React unmount race conditions in FilesPanel/KbPanel, WebSocket exponential backoff, empty dir fallback, debounce timer race
- **Updated** `index.md` — Added source-project-root-v10, updated desktop-app description

## [2026-06-25] ingest | Project Root (.) — v9: KB panel hardening (CSS, security, UX)
- **Created** `source-project-root-v9.md` — Source summary: undefined CSS var fix, path traversal fix, entry sorting, search debounce, stale selection fix
- **Updated** `kb-agent.md` — Expanded bug fixes section with v9 hardening details (6 items)
- **Updated** `index.md` — Added source-project-root-v9, updated kb-agent description

## [2026-06-25] ingest | Project Root (.) — v8: KB panel crash fix, route registration, null tags guard
- **Created** `source-project-root-v8.md` — Source summary: KB route registration bug, null tags guard, desktop KB panel, 806 knowledge cards
- **Updated** `server-websocket.md` — Added 6 KB REST API endpoints (stats, entries, categories, tags, health, read)
- **Updated** `kb-agent.md` — Added Desktop KB Panel section (3 views, 6 backend endpoints, bug fixes), updated source list with kb_handler.go and KbPanel.tsx
- **Updated** `desktop-app.md` — Added KB icon to right sidebar rail (5-icon), added Knowledge Base Panel section
- **Updated** `index.md` — Added source-project-root-v8, updated kb-agent, desktop-app, server-websocket descriptions

## [2026-06-24] ingest | Project Root (.) — v7: mock removed, provider required, test infrastructure cleanup
- **Created** `source-project-root-v7.md` — Source summary: mock provider deletion, registerProviders error on empty, runtime mock fallback removal, test infra updates
- **Updated** `llm-provider-system.md` — Marked Mock as ❌ Removed (v7), updated config section to note PI_GO_PROVIDER is now required
- **Updated** `config-system.md` — Updated Provider struct comment (default "", required at runtime), replaced mock provider option with ❌ Removed (v7) note, added registerProviders error behavior
- **Updated** `overview.md` — Updated provider list from "Anthropic / OpenAI / Mock" to "Anthropic / OpenAI (Mock removed v7, DeepV removed v6)"
- **Updated** `index.md` — Added source-project-root-v7, updated llm-provider-system and config-system descriptions

## [2026-06-23] ingest | Project Root (.) — v6: config overhaul, deepv removed, LoadDotEnv priority fix
- **Created** `source-project-root-v6.md` — Source summary: deepv removal, PI_GO_* env naming, LoadDotEnv no-override, gateway identification
- **Updated** `llm-provider-system.md` — Marked DeepV as removed, updated env var docs to PI_GO_API_KEY
- **Updated** `config-system.md` — Removed DeepV config fields, added LoadDotEnv priority chain, updated env var naming
- **Updated** `overview.md` — Changed provider list from "Anthropic / OpenAI / DeepV / Mock" to "Anthropic / OpenAI / Mock"
- **Updated** `index.md` — Added source-project-root-v6, updated llm-provider-system and config-system descriptions

## [2026-06-10] init | Wiki Initialized
- Created wiki directory structure
- Ready for source ingestion

## [2026-06-10] ingest | Project Root (.)
- **Created** `source-project-root.md` — Source summary of the full project root
- **Created** `agent-core.md` — Agent state machine entity
- **Created** `tool-system.md` — Tool interface and built-in tools entity
- **Created** `llm-provider-system.md` — LLM provider registry entity
- **Created** `session-persistence.md` — JSONL session storage entity
- **Created** `slash-command-framework.md` — 14 slash commands entity
- **Created** `tool-lifecycle-hooks.md` — Before/After hooks entity
- **Created** `extension-system.md` — Plugin extension entity
- **Created** `skill-system.md` — Markdown skill loading entity
- **Created** `four-layer-architecture.md` — Architecture concept
- **Created** `runtime-application-interface.md` — Platform↔App decoupling concept
- **Created** `agent-loop.md` — Dual-loop execution concept
- **Created** `context-compaction.md` — LLM summarization concept
- **Created** `goal-driven-loop.md` — Goal-driven mode concept
- **Created** `operations-abstract.md` — Local/SSH backend concept
- **Updated** `index.md` — Added all new pages to their sections
- **Updated** `overview.md` — (no changes needed, already covered all)

## [2026-06-20] ingest | Project Root (.) — incremental re-ingest
- **Updated** `source-project-root.md` — New date, 14 notable facts (up from 7), added music/web_fetch/loop-detect/MicroCompact/hook facts
- **Created** `music-agent.md` — NetEase Cloud Music application layer with 6 tools, HTTP proxy, per-session extension
- **Created** `web-fetch-tool.md` — 8th built-in tool: URL→markdown with SSRF protection (isPrivateHost, redirect check)
- **Created** `micro-compact.md` — Two-tier compaction concept: MicroCompact (60%, no LLM) → Full Compact (90%, LLM)
- **Updated** `tool-system.md` — Added web_fetch as 8th tool, added music-agent tools section
- **Updated** `context-compaction.md` — Added two-tier strategy, MicroCompact config, MicroCompacted event
- **Updated** `overview.md` — Added music agent to capabilities table, added "两种应用层" section
- **Updated** `index.md` — Added 3 new pages (music-agent, web-fetch-tool, micro-compact)
- **Skipped** `agent-core.md` — Already had loop detection, confirmation, session hooks
- **Skipped** `tool-lifecycle-hooks.md` — Already had session observer hooks

## [2026-06-14] ingest | Project Root (.) — comprehensive re-ingest
- **Created** `desktop-app.md` — Electron + React desktop application entity
- **Created** `feishu-integration.md` — Lark/Feishu bot bridge entity
- **Created** `server-websocket.md` — HTTP REST + SSE + WebSocket server entity
- **Created** `config-system.md` — Environment-driven configuration entity
- **Created** `coding-application.md` — Coding agent application entity
- **Created** `tui-presenter.md` — Terminal UI rendering entity
- **Created** `external-tools.md` — HTTP callback tool registration entity
- **Created** `web-embed.md` — Embedded SPA static file serving entity
- **Created** `ai-transform-retry.md` — Message transform, retry, cost, model concept
- **Updated** `overview.md` — Added external tools, status updates, desktop link
- **Updated** `agent-core.md` — Added dependencies on external-tools and coding-application
- **Updated** `tool-system.md` — Added partitioning details, external tools section
- **Updated** `runtime-application-interface.md` — Added coding-application and server links
- **Updated** `index.md` — Added 9 new pages to entities section

## [2026-06-21] ingest | Project Root (.) — re-ingest v2
- **Created** `source-project-root-v2.md` — Comprehensive re-ingest with new findings
- **Updated** `goal-driven-loop.md` — Added LLM evaluator, dual evaluation system, keyword fallback
- **Updated** `agent-core.md` — Added 15 event types (was 11), loop detection details
- **Updated** `tool-lifecycle-hooks.md` — Added confirmation gate, ToolCallContext, AfterHookError
- **Updated** `overview.md` — Added loop detection, confirmation gate, updated slash command count to 14
- **Updated** `index.md` — Added source-project-root-v2, updated entity descriptions

## [2026-06-22] ingest | Project Root (.) — re-ingest v3
- **Created** `source-project-root-v3.md` — Source summary covering decisions, deployment, desktop internals, agent guidance
- **Created** `personal-assistant-roadmap.md` — Evolution to general personal assistant, memory layer design (OpenViking-inspired), phased rollout P0-P3
- **Created** `deployment-infrastructure.md` — GitHub Actions CI/CD, Ubuntu server systemd deployment, server layout
- **Created** `agent-guidance-system.md` — CLAUDE.md/AGENTS.md coding conventions for AI agents, skill list
- **Created** `competitive-analysis.md` — DeepV feature gap analysis, 30+ features with P0-P3 prioritization
- **Updated** `overview.md` — Added personal assistant direction, deployment link, agent guidance, competitive analysis, corrected tool count (8) and slash command count (15)
- **Updated** `desktop-app.md` — Added PiGoManager internals (subprocess lifecycle, port allocation, health check), update checker (GitHub Releases API), preload IPC bridge, Zustand store details (WebSocket events, tool kind inference)
- **Updated** `four-layer-architecture.md` — Added skills-vs-application decision framework with 3-layer judgment flow, concrete examples, domain infrastructure note
- **Updated** `context-compaction.md` — Added cross-framework comparison table (Pi/CC/Codex/DeepV), design insights, future enhancement roadmap, customInstructions support
- **Updated** `index.md` — Added 4 new pages (personal-assistant-roadmap, deployment-infrastructure, agent-guidance-system, competitive-analysis), updated descriptions

## [2026-06-22] ingest | Project Root (.) — v3 correction: music multi-source + quality filtering
- **Updated** `music-agent.md` — **MAJOR REWRITE**: Added SourceRouter multi-source architecture (NetEase + Bilibili), composite ID format, Bilibili wbi-signed API client, two-gate quality filtering (blacklist + same-name check with two-pass fallback), cross-source VIP fallback, per-source Referer in audio proxy, Range header passthrough. Updated MusicApplication struct from `*netease.Client` to `*music.SourceRouter`.
- **Updated** `source-project-root-v3.md` — Added "Music Multi-Source Implementation" section with 6 code references; added music-agent contradiction note; added bilibili/quality-filtering tags
- **Updated** `overview.md` — Updated music-agent description to "多源音乐（NetEase + Bilibili）"; updated three application layers description

## [2026-06-22] ingest | KB Agent wiki page
- **Created** `kb-agent.md` — Knowledge base retrieval agent (3 tools: kb_search/kb_read/kb_query), agent-lessons repo integration, 507 knowledge cards, 38 project journals
- **Updated** `overview.md` — Added KB agent to capabilities table; updated "三种应用层" from "Future agents" to KB Agent
- **Updated** `four-layer-architecture.md` — Added `agents/kb/` to Application layer diagram; added KB Agent to concrete examples table; added kb-agent to related concepts
- **Updated** `source-project-root-v3.md` — Added kb-agent to cross-references; added "kb-agent was missing" to contradictions section
- **Updated** `index.md` — Added kb-agent entry to entities section

## [2026-06-22] ingest | Project Root (.) — v4: tool execution internals, session management, path clicking
- **Created** `source-project-root-v4.md` — Source summary: tool 9-step execution, session management layers, WebSocket protocol, desktop path clicking, gateway model discovery
- **Created** `session-manager.md` — sessionmgr package: session CRUD, forking, directory-per-session storage, meta.json, relationship to SessionRegistry
- **Updated** `tool-lifecycle-hooks.md` — **MAJOR REWRITE**: Added full 9-step execution flow (Find→EmitStart→Validate→Prepare→BeforeHooks→Confirmation→BuildOnUpdate→Execute→AfterHooks→EmitEnd), error handling table, tool batching (partitionToolCalls), IsError=false design for rejection
- **Updated** `server-websocket.md` — Added WebSocket protocol (client/server message types), file endpoints (GET/PUT), CORS PUT fix, dynamic model discovery from gateway, CreateSession application field, session messages enrichment
- **Updated** `desktop-app.md` — Added path clicking (Markdown.tsx + store.ts two-layer detection), isFilePath validation, extractLocationsFromText 4 patterns, session history restoration on setActive, file pane operations
- **Updated** `session-persistence.md` — Added session-manager to related pages
- **Updated** `index.md` — Added source-project-root-v4, session-manager; updated descriptions for tool-lifecycle-hooks, server-websocket, desktop-app

## [2026-06-23] ingest | Project Root (.) — v5: bilibili-primary music, global player, workspace layout
- **Created** `source-project-root-v5.md` — Source summary: music source strategy pivot, global music player architecture, workspace layout overhaul, Markdown basePath resolution, NetEase audio fix, Bilibili cookie init
- **Updated** `music-agent.md` — **MAJOR UPDATE**: Reversed primary/fallback (bilibili→netease, was netease→bilibili), updated Sources table with new roles, added Bilibili cookie initialization (SPI+search page), added NetEase audio URL strategy (GET+Range, multi-bitrate), added System Prompt v5 section, added source-project-root-v5 cross-reference
- **Updated** `desktop-app.md` — **MAJOR UPDATE**: Added Workspace Layout v5 (right sidebar 4-icon rail, Views dropdown removed), Global Music Player architecture (GlobalMusicBar + store MusicState), Markdown link resolution (basePath), Backend Workspace APIs (list-dir, search-files, read-file), Electron IPC handlers
- **Updated** `overview.md` — Updated music-agent description to "Bilibili 为主力 + NetEase 推荐", updated three application layers description
- **Updated** `index.md` — Added source-project-root-v5, updated music-agent and desktop-app descriptions