---
type: entity
date: 2026-06-22
tags: [kb, agent, application, knowledge-base, search, retrieval]
related: [[runtime-application-interface]], [[coding-application]], [[music-agent]], [[four-layer-architecture]], [[agent-core]], [[personal-assistant-roadmap]]
---

# KB Agent (知识库 Agent)

> The third application layer in pi-go, providing **knowledge base retrieval** over a personal `agent-lessons` repository. Parallel to [[coding-application]] and [[music-agent]] in the [[four-layer-architecture]].
> Second non-coding agent, further validating the [[personal-assistant-roadmap|personal assistant direction]].

## Overview

The KB agent is a [[runtime-application-interface|Application]] implementation that lets users search and retrieve knowledge from a personal knowledge repository (`agent-lessons`). It provides structured search over knowledge cards, project journals, cross-project knowledge base, issue logs, and raw conversation exports.

Unlike coding (interactive editing) or music (playback + streaming), the KB agent is **read-only retrieval** — no writes, no side effects. This makes it the simplest application in the system.

## Architecture

```
┌───────────────────────────────────────────────────┐
│  KBApplication                                     │
│  (implements runtime.Application)                  │
├───────────────────────────────────────────────────┤
│  3 KB Tools                                        │
│  kb_search / kb_read / kb_query                    │
├───────────────────────────────────────────────────┤
│  agent-lessons Repository (~/agent-lessons)        │
│  ┌──────────────────┐  ┌────────────────────┐     │
│  │ doubao-knowledge/ │  │ project-journals/  │     │
│  │ 507 knowledge     │  │ 38 project logs    │     │
│  │ cards + index     │  │ + KNOWLEDGE_BASE   │     │
│  └──────────────────┘  └────────────────────┘     │
│  ┌──────────────────┐  ┌────────────────────┐     │
│  │ issues/           │  │ doubao-export/ +   │     │
│  │ Pitfall records   │  │ chatgpt-export/    │     │
│  └──────────────────┘  │ Raw conversations   │     │
│                         └────────────────────┘     │
└───────────────────────────────────────────────────┘
```

## Knowledge Base Structure

The repository (`~/agent-lessons`, configurable via `PI_GO_KB_REPO_PATH`) contains 5 modules:

| Module | Path | Content | Scale |
|--------|------|---------|-------|
| Knowledge Cards | `doubao-knowledge/` | LLM-compiled knowledge cards with metadata | 507 cards |
| Project Journals | `project-journals/` | Auto-distilled project development logs | 38 projects |
| Cross-Project KB | `project-journals/KNOWLEDGE_BASE.md` | Cross-project experience distilled | 40+ entries |
| Issue Logs | `issues/` | Manually recorded problem-solution pairs | Various |
| Raw Conversations | `doubao-export/` + `chatgpt-export/` | Original conversation records | ~200 conversations |

### Knowledge Cards

Each card in `doubao-knowledge/` has structured metadata (title, category, tags, summary) indexed in `tags-index.json`. The index enables fast structured search without reading all 507 files.

**Categories**: tech (278), work (60), life (51), other (66), english (34), writing (18)

## Tools (3)

| Tool | File | Description |
|------|------|-------------|
| `kb_search` | `kb_search.go` | Search knowledge cards via `tags-index.json`. Supports keyword, tag, and category filtering with weighted scoring. |
| `kb_read` | `kb_read.go` | Read any file in the repository. Supports offset/limit pagination (default 200 lines, 8K char truncation). |
| `kb_query` | `kb_query.go` | Cross-module full-text grep across all 5 modules. Returns top matches sorted by hit count. |

### kb_search — Structured Card Search

Searches the pre-built `tags-index.json` index. Weighted scoring:

| Match Location | Score |
|---------------|-------|
| Title contains query | 3.0 |
| Tag exact match | 2.0 |
| Tag partial match | 1.5 |
| Summary contains query | 1.0 |

Supports combinable filters: `query` + `tag` + `category`. Results sorted by score descending.

### kb_read — File Reader

Reads any file from the repository. Features:
- **Path resolution**: Absolute paths or relative to repo root
- **Pagination**: `offset` (from line N) + `limit` (default 200)
- **Truncation**: Content capped at 8K chars to protect LLM context
- **Header**: Shows file path, total lines, and displayed range

### kb_query — Cross-Module Full-Text Search

Greps across all 5 modules using keyword AND matching (all keywords must appear in the same line). Features:
- **Module filter**: Restrict to `knowledge_base` / `issues` / `journals` / `cards` / `exports`
- **Hit sorting**: Results sorted by hit count descending
- **Configurable limits**: `max_files` per module (default 3)
- **Context**: Shows up to 3 matching lines per file (truncated to 120 chars)

## KBApplication

```go
type KBApplication struct {
    Cfg      config.Config
    RepoPath string  // path to agent-lessons repo, e.g. ~/agent-lessons
}
```

Implements `runtime.Application`:
- `BuildTools()` — Assembles 3 KB tools via `kbtools.BuildList()`
- `BuildPrompt()` — KB-agent system prompt (Chinese, with knowledge base structure and workflow guide)
- `NewSessionExt()` — Per-session `KBSessionExt`
- `ToolNames()` — Returns `["kb_search", "kb_read", "kb_query"]`

## System Prompt

The prompt is generated dynamically in `prompt/prompt.go` with:
- **Knowledge base structure**: Table of 5 modules with paths and scales
- **Workflow guide**: Maps user intents to tool usage patterns
- **Search tips**: Recommended search cascade (kb_search → kb_query → kb_read)
- **Category reference**: The 6 available categories with counts
- **Interaction style**: Chinese, concise, honest about missing knowledge

## Per-Session Extension

`KBSessionExt` implements `runtime.SessionExt`:
- **Goal support**: Set/clear triggers agent rebuild (same pattern as music agent)
- **Single "default" profile**: No profile switching (KB agent has one mode)
- **Rebuild callback**: `SetRebuild(fn)` registered by `AgentSession` after creation

## Integration

Registered in `cmd/pi-agent/main.go` as `"kb"` in the applications map:

```go
application, err := app.New(app.AppOptions{
    Config:      cfg,
    Application: coding.NewCodingApplication(cfg),
    Applications: map[string]runtime.Application{
        "coding": coding.NewCodingApplication(cfg),
        "music":  musicapp.NewMusicApplication(cfg, musicRouter, musicCache),
        "kb":     kbapp.NewKBApplication(cfg, kbRepoPath),
    },
})
```

The `RepoPath` defaults to `~/agent-lessons` but can be overridden via `PI_GO_KB_REPO_PATH` environment variable.

## Design Characteristics

| Aspect | KB Agent | Music Agent | Coding Agent |
|--------|----------|-------------|--------------|
| Tools | 3 (read-only) | 6 (read + play) | 8 (read + write + execute) |
| Side effects | None | Audio streaming | File system + shell |
| External deps | None (local files) | NetEase + Bilibili APIs | LLM providers |
| Complexity | Low | Medium | High |
| Profiles | 1 (default) | 1 (default) | 3 (build/learn/use) |

## Source

- `internal/agents/kb/application.go` — KBApplication
- `internal/agents/kb/session_ext.go` — KBSessionExt
- `internal/agents/kb/prompt/prompt.go` — System prompt builder
- `internal/agents/kb/tools/` — kb_search, kb_read, kb_query
- `cmd/pi-agent/main.go` — Registration as "kb" application

## Related

- [[runtime-application-interface]] — KBApplication implements this interface
- [[coding-application]] — Parallel application layer (primary)
- [[music-agent]] — Parallel application layer (second)
- [[four-layer-architecture]] — KB agent lives in Application layer; simplest of the three
- [[personal-assistant-roadmap]] — KB agent is the knowledge retrieval foundation for personal assistant
