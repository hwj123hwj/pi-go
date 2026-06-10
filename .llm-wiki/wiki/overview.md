---
tags: [pi-go, agent-framework, golang, coding-agent, feishu-bridge]
created: 2026-06-10
updated: 2026-06-10
---

# Pi-Go Overview

## What It Is

Pi-Go is a general-purpose Agent framework implemented in Go (1.22+). Its core design philosophy is an extensible agent base with pluggable application layers. The primary application today is a coding agent (code editing assistant), but the architecture is application-agnostic.

## Architecture

Four-layer dependency hierarchy (strict top-down): Entrypoints (app/ cli/ server/) -> Application (agents/coding/) -> Platform (runtime/) -> Core (agent/ ai/ session/ ...)

## Key Capabilities

- LLM Providers: Anthropic, OpenAI, DeepV, Mock - extensible via plugin registration
- Streaming: SSE event stream with text delta / tool call / error granularity
- Agent Loop: Dual-loop (outer follow-up + inner tool call), sequential + parallel tool execution
- Built-in Tools: 7 tools - bash, read, write, edit, grep, find, ls
- Session: JSONL append-only persistence with tree branching
- Context Compaction: Auto-summarization of long conversations
- Skill System: Loads SKILL.md from .claude/skills/ directory
- HTTP API: RESTful + SSE streaming + WebSocket
- CLI: 14 slash commands
- Profiles: coding / review dual-profile
- SSH Remote: Local/SSH execution backend switching
- Extension System: Tools, commands, event hook injection
- Tool Lifecycle: Before/After hooks + PrepareArguments interface
- Feishu Bridge: Standalone service connecting Agent to Feishu group chat

## Desktop Client

Electron + React + TypeScript app under desktop/. Vite + electron-builder. Supports arm64 and x64.

## Project Stats

~12,400 lines of Go (74 source + 39 test files), Go 1.22+, CI via GitHub Actions.

## Key Config

Provider + API key via .env file. Runtime config via PI_GO_* env vars (20+ knobs). Workspace-relative data directory (./data).

