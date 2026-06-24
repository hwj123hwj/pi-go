---
type: entity
date: 2026-06-24
tags: [kb, agent, application, knowledge-base, second-brain, search, retrieval, save, maintain, stewardship]
related: [[runtime-application-interface]], [[coding-application]], [[music-agent]], [[four-layer-architecture]], [[agent-core]], [[personal-assistant-roadmap]], [[skill-system]]
---

# KB Agent (知识库 Agent / 第二大脑)

> The third application layer in pi-go, providing a **personal second-brain** — search, browse, read, write, and **maintain** knowledge across a configurable personal repository. Parallel to [[coding-application]] and [[music-agent]] in the [[four-layer-architecture]].

## Overview

The KB agent is a [[runtime-application-interface|Application]] implementation that acts as the **owner and steward** of the user's "second brain." It's not just a passive search engine — it actively maintains the knowledge base's health, structure, and growth.

### Three Core Responsibilities

| Responsibility | What | Tools |
|---------------|------|-------|
| **Retrieve** | Search, browse, and read knowledge | `kb_search`, `kb_list`, `kb_read` |
| **Accumulate** | Persist new knowledge from conversations | `kb_save` |
| **Maintain** | Keep the knowledge base healthy and organized | `kb_maintain` |

### Design Philosophy: Three Knowledge Layers

| Layer | System | Purpose | Data |
|-------|--------|---------|------|
| **Code facts** | `.llm-wiki/` + `/wiki` commands | What the current codebase looks like | Auto-ingested from source code |
| **Personal experience** | KB Agent (`agent-lessons`) | Cross-project lessons, pitfalls, tips, conversation knowledge | doubao-knowledge, exports, issues, journals, personal |
| **Project decisions** | `docs/` | Why decisions were made, where the project is going | Human-written design docs |

The KB agent specifically owns the **Personal experience** layer.

## Architecture (v3)

```
┌───────────────────────────────────────────────────────────┐
│  KBApplication (implements runtime.Application)            │
├───────────────────────────────────────────────────────────┤
│  5 KB Tools (Atomic Operations)                            │
│  kb_search / kb_read / kb_list / kb_save / kb_maintain     │
├───────────────────────────────────────────────────────────┤
│  Capability Layer (internal, pluggable)                    │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ Index Engine │  │ Search Strategy│  │ Maintenance Engine│  │
│  │ (index.go)  │  │ (search.go)  │  │ (maintain.go)    │  │
│  │ scan+cache  │  │ pluggable    │  │ dedup+tags+health│  │
│  └─────────────┘  └──────────────┘  └──────────────────┘  │
├───────────────────────────────────────────────────────────┤
│  Knowledge Repository (path configurable)                  │
│  Default: ~/agent-lessons (PI_GO_KB_REPO_PATH override)    │
│  issues/  tech/  work/  life/  ...                         │
└───────────────────────────────────────────────────────────┘
```

### Key Architectural Principles (v3)

1. **Pluggable Search**: The `SearchStrategy` interface decouples *how* we search from *what* we search. Today it's keyword matching; tomorrow it can be vector similarity without touching the tool layer.

2. **Configurable Path**: The repo path is no longer hardcoded. It flows from `Config.KBRepoPath` → `PI_GO_KB_REPO_PATH` env → default `~/agent-lessons`.

3. **Stewardship**: KB Agent is the knowledge base **owner**, not just a visitor. It has maintenance tools to keep the second brain healthy.

4. **Atomic Capabilities**: Each tool is a thin wrapper over a capability. The intelligence is in the capability layer, not the tool layer.

## Configurable Path

```go
// internal/config/config.go
type Config struct {
    ...
    KBRepoPath string  // path to personal knowledge repo
}
```

Resolution order:
1. `Config.KBRepoPath` (set from `PI_GO_KB_REPO_PATH` env)
2. Default: `~/agent-lessons`

## Auto-Index Engine (`index.go`)

Walks the repository and builds `[]Entry` with 30-second caching:

```go
type Entry struct {
    Path     string    // absolute path
    RelPath  string    // relative to repo root
    Title    string    // first heading or frontmatter title
    Category string    // frontmatter category or top-level dir
    Tags     []string  // frontmatter tags or body tags
    Summary  string    // ## 摘要 section or first paragraph
    Source   string    // > 来源 metadata line
    Modified time.Time // file mtime
}
```

### Multi-Format Parsing

| Format | Structure | Detected By |
|--------|-----------|-------------|
| **YAML frontmatter** | `---\ntitle: ...\ntags: [...]\n---` | Leading `---` |
| **Doubao knowledge cards** | `# Title\n> 来源...\n## 摘要\n## 标签` | `> 来源` line + `## 摘要` section |
| **Chat/Doubao exports** | `# Title\n> URL...\n## 👤 User` | `> URL` line |
| **Project journals** | `# Title\n> 自动生成于...` | `> 自动生成于` line |
| **Legacy markdown** | `# Title\n\n**Tags**: a, b` | `**Tags**` line |

All formats auto-detected and normalized into `Entry`.

## Search Strategy (`search.go`)

```go
type SearchStrategy interface {
    Name() string
    Search(entries []Entry, q SearchQuery) []SearchResult
}
```

### KeywordSearcher (default)

Weighted full-text matching:

| Match Location | Weight |
|---------------|--------|
| Title contains keyword | 5.0 |
| Tag exact match | 3.0 |
| Tag partial match | 2.0 |
| Summary contains keyword | 2.0 |
| File path contains keyword | 1.0 |

### Extension Point

Future strategies (e.g., `VectorSearcher`) implement the same interface. The `SearchTool` accepts any strategy via `NewSearchToolWithStrategy()`.

## Maintenance Engine (`maintain.go`)

This is the v3 innovation. The KB agent can now **diagnose and maintain** knowledge base health.

### HealthReport

```go
type HealthReport struct {
    TotalEntries          int
    Categories            int
    Tags                  int
    EntriesMissingSummary []Entry
    EntriesMissingTags    []Entry
    EntriesMissingTitle   []Entry
    DuplicateGroups       []DuplicateGroup
    TagClusters           []TagCluster
}
```

### Maintenance Capabilities

| Capability | Function | What it does |
|-----------|----------|-------------|
| **Health check** | `GenerateHealthReport()` | Full report: metadata gaps, duplicates, tag clusters |
| **Duplicate detection** | `detectDuplicateTitles()` | Finds entries with normalized-identical titles |
| **Tag clustering** | `detectTagClusters()` | Finds tags differing only in case/trivial variations |
| **Category overview** | `CategoryOverview()` | Category distribution |
| **Tag overview** | `TagOverview()` | Tag frequency analysis |

All maintenance operations are **read-only** — they produce recommendations. The AI reviews and acts.

## Tools (5)

| Tool | File | Mode | Description |
|------|------|------|-------------|
| `kb_search` | `kb_search.go` | Read | Delegates to pluggable SearchStrategy (default: keyword) |
| `kb_read` | `kb_read.go` | Read | Read file content with offset/limit pagination |
| `kb_list` | `kb_list.go` | Read | Browse entries, filter by category/tag, sort |
| `kb_save` | `kb_save.go` | **Write** | Save new knowledge entry with auto-generated frontmatter |
| `kb_maintain` | `kb_maintain.go` | **Maintain** | Health check, dedup, tag analysis, stats (NEW) |

### kb_maintain — Stewardship Tool (NEW)

Four actions via the `action` parameter:

| Action | Description |
|--------|-------------|
| `health` | Full health report: metadata gaps + duplicates + tag clusters |
| `duplicates` | Find entries with near-identical titles |
| `tags` | Tag usage frequency + normalization suggestions |
| `stats` | Category and tag distribution overview |

## System Prompt

The prompt positions the KB agent as the **steward** of the second brain with three core duties:
1. **检索 (Retrieve)** — search and read knowledge
2. **积累 (Accumulate)** — proactively save valuable knowledge
3. **维护 (Maintain)** — keep the knowledge base healthy

## Source

- `internal/agents/kb/application.go` — KBApplication
- `internal/agents/kb/session_ext.go` — KBSessionExt
- `internal/agents/kb/prompt/prompt.go` — System prompt builder
- `internal/agents/kb/tools/index.go` — Auto-index engine
- `internal/agents/kb/tools/search.go` — Search strategy interface + KeywordSearcher (NEW)
- `internal/agents/kb/tools/maintain.go` — Maintenance engine (NEW)
- `internal/agents/kb/tools/kb_search.go` — Search tool (refactored to use strategy)
- `internal/agents/kb/tools/kb_read.go` — File reader
- `internal/agents/kb/tools/kb_list.go` — Structured browser
- `internal/agents/kb/tools/kb_save.go` — Knowledge writer
- `internal/agents/kb/tools/kb_maintain.go` — Maintenance tool (NEW)
- `internal/agents/kb/tools/tools.go` — Toolset assembly
- `internal/config/config.go` — `KBRepoPath` config field

## Related

- [[runtime-application-interface]] — KBApplication implements this interface
- [[coding-application]] — Parallel application layer (primary)
- [[music-agent]] — Parallel application layer (second)
- [[four-layer-architecture]] — KB agent lives in Application layer
- [[personal-assistant-roadmap]] — KB agent is the second-brain foundation
