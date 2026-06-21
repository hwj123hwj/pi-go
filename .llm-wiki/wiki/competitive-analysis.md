---
type: concept
date: 2026-06-22
tags: [competitive-analysis, deepv, feature-gap, roadmap, tool-enhancement]
related: [[tool-system]], [[context-compaction]], [[tool-lifecycle-hooks]], [[coding-application]]
---

# DeepV Competitive Analysis

> Comprehensive comparison between pi-go and DeepVcodeClient (commercial coding agent), identifying 30+ feature gaps with prioritized implementation roadmap.

## Overview

| Project | Role | Scale |
|---------|------|-------|
| pi-go (Go) | Our rewrite | Generic agent framework, 7 built-in tools |
| DeepVcodeClient (TypeScript) | Commercial reference | 30+ tools, extensive safety/UX |

## High-Priority Gaps (P0)

### Already Implemented ✅
- **Confirmation gate** — `ToolWithConfirmation` interface for dangerous operations
- **Loop detection** — SHA256 fingerprinting, threshold=5 consecutive identical calls
- **MicroCompact** — Zero-LLM cost tool result cleanup at 60% context threshold
- **Goal evaluator** — LLM-based with keyword fallback

### Still Missing ❌

| Feature | Description | Impact |
|---------|-------------|--------|
| **Edit Corrector** | When `old_string` match fails, suggest closest match via Levenshtein distance | Highest-frequency failure reason in agent editing |
| **ReadManyFilesTool** | Batch file reading with glob patterns, char budget, .gitignore respect | Agent currently reads files one-by-one |
| **AskUserQuestionTool** | Agent can ask user structured questions mid-execution | Agent has no way to proactively clarify |
| **Dangerous command detection** | Blacklist for rm -rf, dd, format etc. | Safety gap |

## Medium-Priority Gaps (P1)

| Feature | Description |
|---------|-------------|
| **LSP tools (6)** | Hover, goto-definition, find-references, document/workspace symbols, implementation |
| **PostCompactRestoration** | Auto-re-read recently accessed files after compaction |
| **TodoWrite tool** | Agent-managed task list |
| **Lint tools** | Auto-lint after edit, auto-fix common issues |
| **Multiple agent styles** | 6 prompt styles (default/codex/cursor/augment/windsurf/antigravity) |
| **Prompt cache boundary** | Static/dynamic prompt separation for provider caching |
| **Tool result separation** | `llmContent` vs `returnDisplay` vs `visualDisplay` |

## Low-Priority Gaps (P2)

WebFetchTool ✅ (implemented), WebSearchTool (decided: skip), BatchTool, MultiEditTool, PatchTool, MCP integration, sandbox detection, project structure detection, FullContext mode, telemetry, policy engine.

## Implementation Roadmap

| Phase | Content | Time | Value |
|-------|---------|------|-------|
| **P0 ✅** | Loop detection + confirmation + MicroCompact + goal evaluator | Done | Agent safety baseline |
| **P1** | Edit Corrector + AskUserQuestion + ReadManyFiles + dangerous command detection | 5-7 days | Fix highest-frequency failures + safety |
| **P2** | PostCompactRestoration + LSP + lint tools + prompt cache | 3-5 days | Context quality + IDE intelligence |
| **P3** | TaskTool/SubAgent + advanced tools | 5-7 days | Complex task decomposition |

## Architecture Recommendations from Analysis

1. **Tool result separation**: Split `ToolResult` into `Content` (for LLM) + `DisplayText` (for user) + `Details` (structured data)
2. **Compression layering**: L0: MicroCompact → L1: FullCompact → L2: PostCompactRestore
3. **Pluggable prompt builder**: Register `PromptSection` interface for composable system prompts
4. **Optional tool interfaces**: `ToolWithKind`, `ToolWithLocations`, `ToolWithAbortSignal` for specialized behaviors

## Source

- [docs/decisions/deepvcode-essence-absorption.md](../../docs/decisions/deepvcode-essence-absorption.md) — Full 30+ page analysis
- [docs/research/cc-haha-web-fetch-analysis.md](../../docs/research/cc-haha-web-fetch-analysis.md) — WebFetch reference (upgraded from DeepV to cc-haha)
