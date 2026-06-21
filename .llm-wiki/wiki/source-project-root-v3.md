---
type: source
source_path: .
date: 2026-06-22
tags: [source, project-root, re-ingest, personal-assistant, roadmap, deployment, competitive-analysis, multi-source, bilibili, quality-filtering]
---

# Source: Project Root (.) — Re-ingest v3

> Comprehensive re-ingest of the pi-go project root, covering documentation, decision records, deployment infrastructure, desktop internals, AND the music multi-source implementation (Bilibili integration + quality filtering).

## New Entities and Concepts

### Decision Records (docs/decisions/)
- **personal-assistant-roadmap.md** — Evolution roadmap: coding → personal assistant, memory layer design, OpenViking integration plan
- **deepvcode-essence-absorption.md** — 30+ feature gap analysis vs DeepVcodeClient, 5-phase implementation roadmap
- **skills-vs-application.md** — Decision framework: Skills (vertical) vs Application (horizontal) expansion
- **goal-compact-cross-framework.md** — Cross-framework comparison of /compact and /goal across Pi/CC/Codex/DeepV

### Deployment
- **deploy.yml** — GitHub Actions: test → build linux/amd64 → tarball → SCP → systemd → health check
- **deploy.md** — Server layout (/opt/pi-go with releases + shared), systemd service files

### Desktop Internals
- **pi-go-manager.ts** — Manages pi-agent subprocess lifecycle (spawn, health check, port allocation)
- **update-checker.ts** — GitHub Releases API polling for version updates
- **preload.ts** — Secure IPC bridge (PiAPI: getServerUrl, startServer, checkForUpdate, pickFolder)
- **store.ts** — Zustand store: REST + WebSocket, session CRUD, model management, i18n, theming

### Agent Guidance
- **CLAUDE.md / AGENTS.md** — Coding conventions for AI agents: think-before-code, simplicity-first, precise-changes, goal-driven-execution

### Music Multi-Source Implementation
- **SourceRouter** (`internal/music/router.go`) — Multiplexes NetEase + Bilibili backends, routes by composite ID (`"netease:12345"`, `"bilibili:BV1xx"`)
- **BilibiliAdapter** (`internal/music/bilibili_adapter.go`) — Wraps `bilibili.Client` implementing `MusicSource`
- **BilibiliClient** (`internal/music/bilibili/client.go`) — wbi-signed API, DASH audio extraction, ranking
- **Quality Filtering** (`internal/music/bilibili/filter.go`) — Two-gate: blacklist (教学/reaction/合集) + same-name check; two-pass fallback
- **Cross-source fallback** (`internal/agents/music/tools/play.go`) — NetEase VIP songs auto-fallback to Bilibili; `PlayDetails.IsFallback` flag
- **Audio proxy** (`internal/music/handler.go`) — Multi-source routing, per-source Referer, Range header passthrough

### Contributor Guide
- **CONTRIBUTING.md** — Full development workflow: environment setup, branch conventions, code standards (Go + TypeScript), git commit format (conventional commits), environment variable reference

## Cross-References

- [[personal-assistant-roadmap]] — Evolution direction and memory layer design
- [[deployment-infrastructure]] — CI/CD pipeline and server deployment
- [[agent-guidance-system]] — AI agent coding conventions
- [[desktop-app]] — Updated with internals (PiGoManager, update checker, store)
- [[goal-driven-loop]] — Updated with cross-framework analysis
- [[context-compaction]] — Updated with cross-framework comparison
- [[four-layer-architecture]] — Updated with skills-vs-application decision framework
- [[coding-application]] — Updated with DeepV competitive analysis status
- [[music-agent]] — **MAJOR UPDATE**: Multi-source (NetEase+Bilibili), SourceRouter, composite IDs, quality filtering, cross-source fallback
- [[kb-agent]] — **NEW**: Knowledge base retrieval agent (3 tools), agent-lessons repo integration

## Contradictions with Existing Wiki

- **Tool count**: README says "8 built-in tools" but source-project-root-v2 says "7". Correct count is **8** (web_fetch was added as the 8th). The v2 summary was written before web_fetch was fully integrated.
- **Slash command count**: v2 says "14", README says "15". The `/context` command may have been added post-v2 ingest. Need to verify.
- **music-agent.md was stale**: Previous version (6/20) described music as "NetEase Cloud Music only" with `*netease.Client`. The actual code has a full multi-source architecture with `SourceRouter`, `BilibiliAdapter`, composite IDs, and quality filtering. **This has been corrected.**
- **kb-agent was missing**: The KB agent (`internal/agents/kb/`) was fully implemented but had no wiki page. **This has been corrected** with a new `kb-agent.md`.
