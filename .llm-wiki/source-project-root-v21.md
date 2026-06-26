# Source: Project Root (.) — v21: Tool output auto-synopsis (context window protection)

> Date: 2026-06-26
> Focus: Automatic synopsis generation for large tool outputs, adapted from OpenViking's tool_result_synopsis + tool_result_store.

## Summary

When a tool returns output larger than 4000 chars, an After-hook automatically generates a structural synopsis (code symbols, markdown headers, JSON keys, or text stats + head/tail excerpt) and replaces the LLM-facing Content. The full output is preserved in UserFacing for the UI.

This directly addresses the #1 source of context window bloat: large tool outputs (file reads, grep results, bash output) consuming thousands of tokens per call.

## Architecture

```
Tool Execute() → returns ToolResult{Content: 8000 chars of file content}
         │
         ▼
SynopsisAfterHook (lifecycle hook)
         │
         ├── len(Content) > 4000?
         │   NO → pass through unchanged
         │   YES →
         │       ├── Copy Content → UserFacing (preserve for UI)
         │       ├── Detect type: code / markdown / json / text
         │       ├── Extract structure:
         │       │   code → imports + function/class signatures
         │       │   markdown → all headers
         │       │   json → top-level keys
         │       │   text → line/char/word counts
         │       ├── Add head + tail excerpts (300 chars each)
         │       └── Replace Content with synopsis (~500 chars)
         │
         ▼
LLM sees: "📋 [输出概览] 原始内容 8000 字符\n📦 代码结构:\n  • func main\n  • func helper\n..."
UI sees: full 8000-char output (from UserFacing)
```

## Comparison with OpenViking

| Aspect | OpenViking | pi-go |
|---|---|---|
| Storage | External store (VikingFS) + ref stub | In-place: UserFacing preserves full output |
| Synopsis generation | `generate_tool_result_synopsis()` — Python, type-aware (CSV/TSV/XML/YAML) | `GenerateSynopsis()` — Go, type-aware (code/markdown/json/text) |
| Trigger threshold | Per-turn budget (`assistant_turn_inline_budget_chars`) | Single threshold (4000 chars per tool result) |
| Integration | Session `_externalize_large_tool_output_group` | Lifecycle `After` hook |
| Turn-level budget | Yes: iteratively externalize until under budget | No: per-result threshold (simpler, sufficient) |

## Implementation

### New: `internal/agent/synopsis.go`

- `SynopsisAfterHook` — implements `AfterToolCallHook`, registered globally in agent session
- `GenerateSynopsis(content)` — main entry point, detects type and delegates
- `detectContentType()` — pattern matching for code/markdown/json/text
- `summarizeCode()` — extracts function/class/import signatures via regex
- `summarizeMarkdown()` — extracts all `#` headers
- `summarizeJSON()` — extracts top-level keys
- `summarizeText()` — line/char/word statistics

### New: `internal/agent/synopsis_test.go`

10 tests: hook behavior (small/large), content type detection (go/python/json/markdown/text), code summary (imports + functions), markdown summary (headers), synopsis generation (code + json), truncation helpers.

### Modified: `internal/runtime/agent_session.go`

- Added `agent.SynopsisAfterHook` to the lifecycle hooks chain in `buildAgent()`
- One line: `lifecycleHooks.After = append(lifecycleHooks.After, agent.SynopsisAfterHook)`

## Impact

Before: A `kb_read` or `bash` tool returning 8000 chars → 8000 chars (~2000 tokens) in context.
After: Same tool → ~500 chars (~125 tokens) synopsis + full output visible to user.
**Savings: ~94% per large tool call.**

## Cross-references
- [[source-project-root-v18]] — KB L1 overview mode (manual synopsis via overview=true)
- [[source-project-root-v20]] — Session memory extraction
- [[tool-system]] — Tool interface and lifecycle hooks
- [[tool-lifecycle-hooks]] — AfterToolCallHook design
