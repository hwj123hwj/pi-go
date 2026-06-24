---
type: entity
date: 2026-06-24
tags: [kb, agent, application, knowledge-base, second-brain, search, retrieval, save]
related: [[runtime-application-interface]], [[coding-application]], [[music-agent]], [[four-layer-architecture]], [[agent-core]], [[personal-assistant-roadmap]], [[skill-system]]
---

# KB Agent (知识库 Agent / 第二大脑)

> The third application layer in pi-go, providing a **personal second-brain** — search, browse, read, and write knowledge across a personal `agent-lessons` repository. Parallel to [[coding-application]] and [[music-agent]] in the [[four-layer-architecture]].

## Overview

The KB agent is a [[runtime-application-interface|Application]] implementation that turns the `agent-lessons` repo into an AI-queryable "second brain." Unlike the previous read-only version, it now supports **writing** new knowledge entries, making it a true read-write knowledge assistant.

### Design Philosophy: Three Knowledge Layers

| Layer | System | Purpose | Data |
|-------|--------|---------|------|
| **Code facts** | `.llm-wiki/` + `/wiki` commands | What the current codebase looks like | Auto-ingested from source code |
| **Personal experience** | KB Agent (`agent-lessons`) | Cross-project lessons, pitfalls, tips | Manually/AI-written knowledge cards |
| **Project decisions** | `docs/` | Why decisions were made, where the project is going | Human-written design docs |

The KB agent specifically owns the **Personal experience** layer.

## Architecture (v2)

```
┌───────────────────────────────────────────────────────┐
│  KBApplication (implements runtime.Application)        │
├───────────────────────────────────────────────────────┤
│  4 KB Tools                                            │
│  kb_search / kb_read / kb_list / kb_save               │
├───────────────────────────────────────────────────────┤
│  Auto-Index Engine (index.go)                          │
│  Scans repo → in-memory index (cached 30s)             │
│  Parses YAML frontmatter + legacy markdown             │
├───────────────────────────────────────────────────────┤
│  agent-lessons Repository (~/agent-lessons)            │
│  Flat or nested directories of .md files               │
│  issues/  tech/  work/  life/  ...                     │
└───────────────────────────────────────────────────────┘
```

### Key Change: No External Index File

The old version depended on a pre-built `tags-index.json`. The new version builds an **in-memory index automatically** by scanning all `.md` files in the repo. This means:

- No pre-processing step required
- Works with any directory structure
- Supports both YAML frontmatter and legacy markdown formats
- 30-second cache to balance freshness vs performance

## Auto-Index Engine (`index.go`)

The core innovation of v2. Walks the repository and builds `[]Entry`:

```go
type Entry struct {
    Path     string    // absolute path
    RelPath  string    // relative to repo root
    Title    string    // first heading or frontmatter title
    Category string    // frontmatter category or top-level dir
    Tags     []string  // frontmatter tags or #tags / **Tags**: in body
    Summary  string    // first paragraph or frontmatter summary
    Modified time.Time // file mtime
}
```

### Dual Format Parsing

| Format | Structure | Example |
|--------|-----------|---------|
| **YAML frontmatter** | `---\ntitle: ...\ntags: [...]\n---` | Modern entries with structured metadata |
| **Legacy markdown** | `# Title\n\n**Tags**: a, b\n\nbody` | Older entries without frontmatter |

Both formats are auto-detected and normalized into `Entry`.

## Tools (4)

| Tool | File | Mode | Description |
|------|------|------|-------------|
| `kb_search` | `kb_search.go` | Read | Weighted full-text search across title/tags/summary/path |
| `kb_read` | `kb_read.go` | Read | Read file content with offset/limit pagination |
| `kb_list` | `kb_list.go` | Read | Browse all entries, filter by category/tag, sort by recent/title |
| `kb_save` | `kb_save.go` | **Write** | Save a new knowledge entry with auto-generated frontmatter |

### kb_search — Weighted Full-Text Search

No longer depends on `tags-index.json`. Uses the auto-index. Weighted scoring:

| Match Location | Weight |
|---------------|--------|
| Title contains keyword | 5.0 |
| Tag exact match | 3.0 |
| Tag partial match | 2.0 |
| Summary contains keyword | 2.0 |
| File path contains keyword | 1.0 |

Supports combinable filters: `query` + `tag` + `category`.

### kb_list — Structured Browser (replaces kb_query)

The old `kb_query` was a grep-based cross-module search. It's been replaced by `kb_list` which provides a structured overview:

- Browse all entries grouped by category
- Filter by `category` or `tag`
- Sort by `recent` (mtime) or `title`
- Configurable `limit`

### kb_save — Second Brain Write Capability (NEW)

The most significant addition. The AI can now **persist knowledge** to the repo:

- Auto-generates YAML frontmatter (title, date, category, tags)
- Auto-generates filename: `YYYY-MM-DD-slug.md` (hash-based slug for non-ASCII titles)
- Supports category-based directory routing (`issues/`, `tech/`, etc.)
- Collision avoidance (appends -2, -3, ...)
- Invalidates index cache after save
- Designed for "记住这个" / "记录一下" / "save this" user intents

### kb_read — File Reader (enhanced)

Same core functionality but with improved path resolution:
- Accepts relative paths (resolved against repo root)
- Accepts absolute paths
- Pagination via offset/limit
- 8K char truncation for LLM context safety

## System Prompt

The prompt was rewritten from scratch. Key changes:
- Removed all hardcoded data numbers (507 cards, 38 projects, etc.) — the AI discovers content dynamically
- Removed references to non-existent directories (`doubao-knowledge/`, `doubao-export/`, etc.)
- Positioned as "第二大脑" (second brain) rather than just a retrieval tool
- Added kb_save workflow guidance (proactive knowledge accumulation)

## Source

- `internal/agents/kb/application.go` — KBApplication
- `internal/agents/kb/session_ext.go` — KBSessionExt
- `internal/agents/kb/prompt/prompt.go` — System prompt builder
- `internal/agents/kb/tools/index.go` — Auto-index engine (NEW)
- `internal/agents/kb/tools/kb_search.go` — Weighted search (rewritten)
- `internal/agents/kb/tools/kb_read.go` — File reader (enhanced)
- `internal/agents/kb/tools/kb_list.go` — Structured browser (NEW, replaces kb_query)
- `internal/agents/kb/tools/kb_save.go` — Knowledge writer (NEW)
- `internal/agents/kb/tools/tools.go` — Toolset assembly

## Related

- [[runtime-application-interface]] — KBApplication implements this interface
- [[coding-application]] — Parallel application layer (primary)
- [[music-agent]] — Parallel application layer (second)
- [[four-layer-architecture]] — KB agent lives in Application layer
- [[personal-assistant-roadmap]] — KB agent is the second-brain foundation
