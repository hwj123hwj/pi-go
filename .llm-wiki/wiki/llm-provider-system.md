---
type: entity
date: 2026-06-10
tags: [provider, llm, ai, registry]
---

# LLM Provider System

> Pluggable LLM backend system using a registry pattern.

## Provider Registry

The `providers.Registry` uses plugin-style registration:
- `Register(name, provider)` — Register a provider
- `Get(name)` — Lookup by name (returns `(Provider, bool)`)
- Built-in providers registered in `register.go`

## Supported Providers

| Provider | Status | Source |
|----------|--------|--------|
| `mock` | ✅ Built-in | For testing without real LLM |
| `anthropic` | ✅ Built-in | Anthropic Messages API |
| `openai` | ✅ Built-in | OpenAI Chat Completions API |
| `deepv` | ✅ Built-in | Custom DeepV server |

## Provider Interface

The `Provider` interface (in `internal/ai/providers/`):
- `Name()` — Provider identifier
- `Stream()` — Streaming chat completion
- `StreamSimple()` — Simplified streaming

## Configuration

Configured via environment variables:
- `PI_GO_PROVIDER` — Select provider
- `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `DEEPV_SERVER_URL`
- Model names, base URLs, etc.

## Related

- [[agent-core]] — Agent uses Registry to resolve providers
- [[tool-system]] — Tool definitions are sent as part of LLM requests