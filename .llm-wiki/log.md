# LLM Wiki Log

> Chronological record of wiki operations.

## [2026-07-05] ingest | Web Search + Session Hardening + Undo System
- **Created** `wiki/web-search-undo-session.md` — Three features: (1) web_search tool with dual engine (SearXNG JSON API + DuckDuckGo HTML fallback, SSRF protection, zero-config DDG); (2) session crash-safety (fsync after append, crypto/rand ID replacing time.Nano()); (3) undo/rollback with BackupManager (per-file snapshot stack, auto-snapshot in edit/write, /undo slash command)
- **Updated** `index.md` — Added `[[web-search-undo-session]]`
- **Key files**: `internal/tools/web_search.go`, `internal/tools/backup.go`, `internal/session/jsonl.go`, `internal/agents/coding/commands/undo.go`, `internal/agents/coding/tools/tools.go`

## [2026-07-05] ingest | Model Registry & Feishu Multi-Chat Fix
- **Created** `wiki/model-registry-feishu-fix.md` — Model registry architecture (JSON config, 13 built-in models, provider-based filtering) + Feishu sender race fix (per-chat map replacing single shared field, session_id in tool callback)
- **Updated** `index.md` — Added `[[model-registry-feishu-fix]]` to Concepts section
- **Key files**: `internal/models/registry.go`, `internal/models/registry_test.go`, `internal/agents/coding/application.go`, `internal/feishu/handler.go`, `internal/feishu/tool.go`

## [2026-07-05] ingest | Loop Scheduler & Task Handoff (absorbed from hwjcode)
- **Created** `wiki/loop-scheduler.md` — New concept page: /loop watchdog architecture (goroutine + time.Ticker, TriggerResolver pattern, per-session loops), TASK.md handoff design (document-as-context philosophy, auto-load in prompt builder, structured Markdown format)
- **Updated** `index.md` — Added `[[loop-scheduler]]` to Concepts section
- **Key files**: `internal/scheduler/loop.go`, `internal/scheduler/duration.go`, `internal/handoff/task.go`, `internal/agents/coding/commands/loop.go`, `internal/agents/coding/commands/task.go`, `internal/agents/coding/prompt/builder.go` (auto-inject), `internal/server/server.go` (TriggerResolver wiring)

## [2026-06-28] ingest | Voice Input / ASR Feature (v40)
- **Created** `wiki/source-project-root-v40.md` — Source summary: full-stack ASR pipeline (backend proxy, frontend hook, config fields, Android permission)
- **Created** `wiki/asr-voice-input.md` — New concept page: architecture diagram, design decisions (server proxy, API key reuse, codec selection, toggle interaction), file list
- **Updated** `wiki/desktop-app.md` — Added Voice Input / ASR (v40) section: PromptBar 🎤 button, useVoiceInput hook, desktop + mobile sizing, RECORD_AUDIO permission. Updated tags (+asr, +voice-input)
- **Updated** `wiki/server-websocket.md` — Added `POST /asr/transcribe` to REST API table, added `/asr/*` to route hierarchy, updated tags (+asr), updated related (+asr-voice-input)
- **Updated** `wiki/config-system.md` — Added ASR config fields (ASRAPIKey, ASRModel, ASRBaseURL) to struct and env var docs, added related link
- **Updated** `index.md` — Added source-project-root-v40, asr-voice-input; updated desktop-app, server-websocket, config-system descriptions
- **Contradictions**: None. New feature, no existing content contradicted.

## [2026-06-28] ingest | Mobile Self-Update Polish + Version Management (v39)
- **Created** `wiki/source-project-root-v39.md` — Source summary covering: manual check-update button in sidebar footer, model selector restored on mobile (removed isElectron guard), music bar transform fix, version bump to 0.7.0
- **Updated** `wiki/desktop-app.md` — Added 5 new sections: Mobile Self-Update System (v38), Manual Check-Update Button (v39), Version Management for Self-Update (v39), Model Selector Restored on Mobile (v39), Global Music Bar Transform Fix (v39), Sidebar Footer Check-Update Button CSS (v39). Updated date to 2026-06-28, added tags (mobile, capacitor, self-update, version-management)
- **Updated** `index.md` — Added source-project-root-v39 entry, updated desktop-app description
- **Contradictions flagged**: (1) Model selector was documented as "Hidden (saves vertical space)" in v27 table — now restored, documented in v39 section. (2) Music bar mobile layout described as working in v27 — transform bug existed since v27, fixed in v39.

## [2026-06-27] feat | Mobile Self-Update + v0.6.0 Release (v38)
- **Updated** `wiki/desktop-app.md` — Major feature: in-app self-update for Android:
  - Custom Capacitor plugin `ApkUpdaterPlugin.java`: native APK download (with progress events via `notifyListeners`) + FileProvider URI + `ACTION_VIEW` install intent
  - `mobile-updater.ts`: TypeScript wrapper, GitHub Releases API check, semver comparison, `downloadAndInstallApk()` with progress callback
  - `MobileUpdateDialog.tsx`: update dialog with version info, download progress bar, "稍后" / "下载并安装" buttons
  - Android config: `REQUEST_INSTALL_PACKAGES` permission, FileProvider `apk_downloads` path, version bump to 0.6.0 (versionCode 6)
  - Toolbar declutter: density toggle + sidebar/bottom toggles hidden on mobile, only right-panel button shown, `justify-content: space-between`

## [2026-06-27] fix | Mobile Toolbar Declutter (v37)
- **Updated** `wiki/desktop-app.md` — Mobile toolbar was overcrowded: title + status dot + density toggle (3 buttons) + 3 workspace toggles (sidebar/bottom/right) = 8 elements in 48px height. Fixed: (1) Density toggle hidden entirely on mobile (`toolbar-density-mobile` → `display: none`); (2) WorkspaceToggles component: sidebar toggle and bottom terminal toggle hidden on mobile (`isElectron` gated), only right panel toggle shown; (3) Toolbar uses `justify-content: space-between` — title on left, right-panel toggle on right; (4) Removed duplicate `.workspace-toggles` CSS block.

## [2026-06-27] fix | Mobile Code Review Round 4 — Bug Fixes (v36)
- **Updated** `wiki/desktop-app.md` — Code review round 4 found and fixed 2 bugs:
  1. **`.rsidebar-content` overflow conflict** — Desktop definition sets `overflow: hidden` (line 494), mobile sets `overflow-y: auto` (line 5227). Without `!important`, the desktop `overflow: hidden` could prevent scrolling inside the right sidebar on mobile (e.g. long file content, KB articles). Fixed: `overflow-y: auto !important` on mobile.
  2. **CodeBlock copy button unhandled promise rejection** — `navigator.clipboard?.writeText(code).then(...)` had no `.catch()` handler. On mobile browsers with insecure origins (HTTP without HTTPS), the Clipboard API throws `NotAllowedError`, causing an unhandled promise rejection. Fixed: added `.catch()` to silently ignore.

## [2026-06-27] fix | Mobile Code Review Round 3 — Bug Fixes (v35)
- **Updated** `wiki/desktop-app.md` — Code review round 3 found and fixed 2 bugs:
  1. **Sidebar backdrop double-dimming on mobile** — The app has both a `::before` pseudo-element overlay AND a `.sidebar-mobile-backdrop` div, both at 50% black. With `pointer-events: none` on `::before`, clicks passed through to the underlying app. Fixed: keep `::before` as `pointer-events: none` but make it transparent when `.sidebar-mobile-backdrop` is present (via `:has()` selector) to prevent double-dimming.
  2. **`.btn-stop` mobile CSS missing `!important`** — `width: 36px`, `height: 36px`, `font-size: 0`, `gap: 0` lacked `!important` and could be overridden by the base `.btn-stop` definition (which sets `height: 32px`, `padding: 0 12px`, `font-size: 12.5px`). Fixed: all critical properties now have `!important`.

## [2026-06-27] fix | Mobile Code Review Round 2 — Bug Fixes (v34)
- **Updated** `wiki/desktop-app.md` — Code review round 2 found and fixed 2 bugs:
  1. **File tree not hidden on mobile** — CSS targeted `.files-sidebar` class that doesn't exist in the DOM; the actual elements are `.file-tree` and `.resizer` rendered directly inside `.files-body`. Fixed selectors to `.files-panel .file-tree` + `.files-body > .resizer`.
  2. **ChatPane onTouchStart steals focus from interactive elements** — The keyboard-dismiss `onTouchStart` handler called `.blur()` on any touch, including taps on buttons (code copy, tool card expand, etc.). Fixed with `target.closest()` guard to skip interactive elements.

## [2026-06-27] fix | Mobile Code Review — Bug Fixes (v33)
- **Updated** `wiki/desktop-app.md` — Code review of commits v27–v32 found and fixed 4 bugs:
  1. **Duplicate `.toolbar-density-mobile` CSS** — Two blocks defined with conflicting properties. Consolidated to one in the mobile media query with `display: flex` + `margin-left: auto`.
  2. **Duplicate `.rsidebar-content` CSS** — Three definitions existed (desktop + two mobile). Merged into one mobile definition with `padding-top` for the close button clearance.
  3. **`GlobalMusicBar.close` didn't clear audio src** — Calling `audio.pause()` + `clearMusic()` left the `src` attribute intact, causing the browser to keep the media resource loaded. Fixed: now calls `audio.removeAttribute('src')` + `audio.load()` to fully release.
  4. **Unused `playMusic` variable in GlobalMusicBar** — After v30 refactor, `playMusic` was subscribed but never used, causing unnecessary store re-renders. Removed.

## [2026-06-27] ingest | Mobile Toolbar + Empty State + Keyboard Dismiss (v32)
- **Updated** `wiki/desktop-app.md` — Added v32 mobile optimizations: (1) Density toggle on mobile now uses `toolbar-density-mobile` class — compact 28px buttons instead of hidden; (2) Empty state hides folder picker and model selector on mobile (`isElectron` gated) — mobile users get a clean single-purpose prompt + send button; (3) Keyboard auto-dismiss — touching/scrolling the transcript blurs active input (textarea/input) to hide the soft keyboard, improving scroll UX; (4) Empty state send button: 44px min touch target

## [2026-06-27] ingest | Mobile Panel Close Button + PromptBar + File Panel (v31)
- **Updated** `wiki/desktop-app.md` — Added v31 mobile optimizations: (1) Right sidebar mobile close button — floating 40px back-arrow in top-left of any right sidebar panel, calls `toggleWorkspaceRight()` to close; `rsidebar-content` gets top padding to avoid overlap; (2) PromptBar stop button mobile — circular 36px icon-only (removed text label to match send button size); (3) File panel mobile details (file tree, tabs, code view touch)

## [2026-06-27] ingest | Mobile File Access + Music Bar Close (v30)
- **Updated** `wiki/desktop-app.md` — Added v30 mobile fixes: (1) Right sidebar CSS class name mismatch fixed — `.right-sidebar` → `.rsidebar` for full-screen overlay, rail icons now show labels in column layout at bottom with 44px touch targets; (2) File panel mobile optimizations — file tree hidden on mobile, file tabs touch-friendly (opacity:1 close buttons), compact code view; (3) Global music bar close button added (✕ button, calls `clearMusic()` to stop playback and hide bar), 32px touch target on mobile; (4) i18n: `music.close` key added (EN: "Close", ZH: "关闭")

## [2026-06-27] ingest | Mobile UX Overhaul Round 3 (v29)
- **Updated** `wiki/desktop-app.md` — Added Round 3 mobile optimizations: Diff preview mobile (gutter hidden, compact font), tool body compact (max-height 200px, momentum scroll), modal/dialog full-screen mobile (94vw, 40px button height), deep Markdown mobile pass (headings, lists, links word-break), code block copy button + long code collapse/expand feature (CodeBlock component in Markdown.tsx), session list 56px touch height, transcript/sidebar momentum scroll + overscroll-contain, role-tag compact, typing-dots compact, server connect screen iOS zoom prevention + 48px touch button, toolbar status text hidden (dot only)
- **Updated** `index.md` — Added source-project-root-v29 entry

## [2026-06-27] ingest | Mobile UX Overhaul Round 2 (v28)
- **Updated** `wiki/desktop-app.md` — Expanded mobile optimization table (v28): chat transcript density (padding, message spacing, user message radius, assistant text readability), tool card mobile touch targets (44px HIG minimum, status icon-only), inline music card compact mode (hidden progress bar), Markdown mobile rendering (table horizontal scroll, code block radius/height), empty state input iOS zoom prevention, sidebar search/head safe-area, right sidebar rail 44px buttons, `100dvh` keyboard-adaptive viewport
- **Updated** `index.md` — Added source-project-root-v28 entry

## [2026-06-27] ingest | Mobile UX Overhaul (v27)
- **Updated** `wiki/desktop-app.md` — Added comprehensive Mobile / Capacitor Platform section (v27): Server Connect flow, platform detection (`isElectron` / `isRemotePlatform` / `body.mobile`), mobile-specific optimization table (PromptBar, MusicBar, Sidebar, Toolbar, touch targets), audio streaming architecture (relative proxy URLs, `rewriteAudioURL`, Content-Type rejection in handler), Capacitor config details
- **Updated** `index.md` — Added source-project-root-v27 entry

## [2026-06-27] ingest | Project Root (.) — v26: Desktop UI fixes, KB path fix, entrypoint cleanup
- **Created** `wiki/source-project-root-v26.md` — Source summary covering 4 commits: README sync, cmd/pi-music removal, KB absolute path output, desktop 4 UI fixes (bottom terminal, sidebar toggle, file menu overflow, profile error state)
- **Updated** `wiki/four-layer-architecture.md` — Removed `cmd/pi-music` from Entrypoints layer diagram; updated date
- **Updated** `wiki/music-agent.md` — Marked `cmd/pi-music/` source as removed v26
- **Updated** `wiki/desktop-app.md` — Added Bottom Terminal Panel section (v26), Sidebar Toggle behavior (v26), Profile Panel error state (v26), updated Profile Panel heading from "NEW v25" to "v25"
- **Updated** `wiki/kb-agent.md` — Added v26 Bug Fixes section (absolute path output for clickable desktop links, 6 files updated)
- **Updated** `wiki/overview.md` — Updated slash command count 15→16, code stats to "~26,000 行 (144 源文件 + 67 测试文件)", removed cmd/pi-music from entrypoints, added note about pi-music removal, updated date
- **Updated** `index.md` — Added source-project-root-v26
- **Contradictions flagged & resolved**: (1) cmd/pi-music referenced in 3 wiki pages but now deleted — all updated. (2) Slash command count was 15 in overview but README says 16 — updated. (3) LOC stats were outdated — updated from ~14k/54 to ~26k/67.

## [2026-06-27] ingest | Project Root (.) — v25: Full re-ingest (v17–v25: profile, vector search, memory extraction, synopsis, desktop panel)
- **Created** `wiki/source-project-root-v25.md` — Source summary covering unified profile, KB vector search, memory extraction, tool synopsis, desktop profile panel, 3 code review rounds
- **Created** `wiki/unified-profile.md` — New concept page: cross-agent persistent profile store, hotness eviction, fixed-size summary, REST API, desktop panel
- **Created** `wiki/kb-vector-search.md` — New concept page: hybrid keyword+vector search, SiliconFlow bge-m3, local JSON vector store, cosine similarity
- **Created** `wiki/session-memory-extraction.md` — New concept page: async LLM-based fact extraction, OpenViking ExtractLoop adaptation
- **Created** `wiki/tool-output-synopsis.md` — New concept page: After-hook for large output synopsis, content-type detection, double-synopsis prevention
- **Updated** `wiki/desktop-app.md` — Added Profile Panel (6th rail icon), updated tags, updated RightView type, added related link to [[unified-profile]]
- **Updated** `wiki/server-websocket.md` — Added `GET /profile` and `DELETE /profile` endpoints to REST table, updated route hierarchy, updated tags
- **Updated** `wiki/kb-agent.md` — Updated Search Strategy section (vector search now implemented), updated kb_read description (L1 overview mode), added tags, added related links
- **Updated** `wiki/personal-assistant-roadmap.md` — Marked P0/P1/P1.5 as ✅ done, added implementation note about interface design evolution, added related links, updated tags
- **Updated** `wiki/config-system.md` — Added KB config fields (KBRepoPath, KBEmbeddingAPIKey, KBEmbeddingBaseURL, KBEmbeddingModel), added KB/user-profile config sections, added related links
- **Updated** `index.md` — Added 4 new concept pages to Concepts section, updated entity descriptions (kb-agent, desktop-app, server-websocket, config-system), updated personal-assistant-roadmap description
- **Contradictions flagged**: 5 items (roadmap P1 status, kb-agent vector search future tense, server endpoints table, desktop rail count, config KB fields) — all resolved in this ingest

## [2026-06-25] ingest | Project Root (.) — v15: Music player deep-audit round 2
- **Created** `source-project-root-v15.md` — 5 issues: FD leak (idle conn limits), 403 stale detection, retry exhaustion guard, multi-try cross-source fallback, React audio lifecycle race condition
- **Updated** `index.md` — Added source-project-root-v15

## [2026-06-25] ingest | Project Root (.) — v14: Music player robustness fixes
- **Created** `source-project-root-v14.md` — 6 music bugs fixed: stale URL cache retry, lyrics JSON injection, cache memory growth, dead code, audio element src reload, error recovery button
- **Updated** `index.md` — Added source-project-root-v14

## [2026-06-25] ingest | Project Root (.) — v13: Workspace inline file editor
- **Created** `source-project-root-v13.md` — New feature: inline file editing in FilesPanel with PUT /workspace/write-file backend endpoint
- **Updated** `index.md` — Added source-project-root-v13

## [2026-06-25] ingest | Project Root (.) — v12: Desktop chat smart scroll & KB tag view loading
- **Created** `source-project-root-v12.md` — Source summary: chat auto-scroll hijacking user scroll position, KB tag view empty list during loading, tag back button tooltip
- **Updated** `index.md` — Added source-project-root-v12

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