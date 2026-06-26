---
type: concept
date: 2026-06-27
tags: [synopsis, context-window, tool-output, hook, openviking]
related: [tool-lifecycle-hooks, agent-core, kb-agent]
---

# Tool Output Auto-Synopsis

> An `AfterToolCallHook` that automatically replaces large tool outputs (>4000 chars)
> with a deterministic structural synopsis, preserving the full text for the UI.

## Overview

Implemented in v21, adapted from OpenViking's `tool_result_synopsis.py`. When a tool returns output larger than the synopsis threshold, the hook replaces the LLM-facing `Content` field with a compact synopsis while preserving the full output in `UserFacing` for the desktop UI to display.

This prevents large tool outputs (file reads, search results, command output) from consuming context window tokens.

## Mechanism

### Hook Flow

```
Tool.Execute() → ToolResult{Content: "very long output...", UserFacing: ""}
  └─ SynopsisAfterHook (AfterToolCallHook)
       ├─ len(Content) > 4000? → No → pass through
       ├─ Content contains "[输出概览]"? → Yes → skip (double-synopsis guard)
       ├─ Copy Content → UserFacing (if UserFacing is empty)
       └─ GenerateSynopsis(Content) → replace Content
```

### Synopsis Structure

The synopsis includes:
1. **Original size indicator**: `📋 [输出概览] 原始内容 12345 字符`
2. **Content-type-aware structure extraction**:
   - **Code**: imports + function/class/type definitions
   - **Markdown**: headers hierarchy + section count
   - **JSON**: top-level keys + array lengths
   - **Text**: paragraph count + key sentences
3. **Head + tail excerpt** (300 chars each)
4. **Hint**: `💡 完整内容已对用户可见。如需查看完整内容请告知。`

### Content Type Detection

```go
func detectContentType(content string, lines []string) string
```

| Type | Detection Signals |
|------|-------------------|
| `code` | `func `, `package `, `import `, `class `, `def `, `fn `, `pub fn` |
| `json` | Starts with `{`/`[` and ends with `}`/`]` |
| `markdown` | ≥3 lines starting with `#` |
| `text` | Fallback |

## Double-Synopsis Prevention (v23 fix)

When a tool like `kb_read overview=true` already returns structured output (a synopsis), the hook checks for the `synopsisSkipMarker` (`[输出概览]`) in the content. If found, the hook skips synopsis generation.

```go
if strings.Contains(result.Content, synopsisSkipMarker) {
    return result, nil // skip double-synopsis
}
```

## Constants

| Constant | Value | Purpose |
|----------|-------|---------|
| `synopsisThreshold` | 4000 chars | Outputs above this get synopsized |
| `synopsisSkipMarker` | `[输出概览]` | Double-synopsis guard |
| `maxExcerptChars` | 600 | Head + tail excerpt total |
| `maxHeadersShown` | 10 | Max markdown headers in synopsis |
| `maxCodeSymbols` | 15 | Max code symbols in synopsis |

## Key Differences from OpenViking

| OpenViking | pi-go |
|-----------|-------|
| Stores full text to VikingFS with a reference | Full text stays in `UserFacing` (in-memory) |
| Requires external storage backend | No external storage needed |
| LLM-based synopsis | Pure deterministic rules (no LLM call) |

## Code Locations

| File | Responsibility |
|------|----------------|
| `internal/agent/synopsis.go` | Hook, synopsis generation, content detection |

## Source

- [[source-project-root-v21]] — Initial implementation
- [[source-project-root-v23]] — Double-synopsis prevention fix
- [[source-project-root-v22]] — UTF-8 truncation fixes in synopsis

## Related

- [[tool-lifecycle-hooks]] — Hook system that triggers synopsis
- [[kb-agent]] — `kb_read` overview mode uses the same skip marker
- [[agent-core]] — Agent loop integrates the hook
