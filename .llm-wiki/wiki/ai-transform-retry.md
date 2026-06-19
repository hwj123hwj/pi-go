---
type: concept
date: 2026-06-14
tags: [ai, transform, retry, message, streaming]
---

# AI Message Transform & Retry System

> Supporting subsystems in `internal/ai/` for message transformation and retry logic.

## Message Transformation

Source: `internal/ai/transform.go`

Transforms are applied to messages before sending to LLM providers:

| Transformation | Purpose |
|----------------|---------|
| **Image downgrade** | Converts base64 images to URL references for providers with size limits |
| **ID normalization** | Ensures consistent message IDs across providers |
| **Role merging** | Merges consecutive same-role messages (e.g., multiple user messages) |
| **Token count reduction** | Strips unnecessary whitespace |

## Retry Logic

Source: `internal/ai/retry.go`

```go
func RetryStream(ctx context.Context, attemptFunc func(ctx context.Context) error, opts ...RetryOption) error
```

Features:
- **Exponential backoff** with jitter
- **Configurable max retries** (default: 3)
- **Context-aware** — respects context cancellation
- **Error classification** — only retries on transient errors (network errors, rate limits, 5xx)

## Cost Calculation

Source: `internal/ai/cost.go`

```go
func CalculateCost(modelID string, inputTokens, outputTokens int) Cost
```

- Looks up model pricing from a built-in table
- Supports different input/output token pricing
- Returns `Cost{Currency, InputCost, OutputCost, TotalCost}`

## Model Context Window Registry

Source: `internal/ai/models/registry.go`

A lookup table mapping model IDs to their context window sizes (token limits):
- Used by [[context-compaction]] to decide when to compact
- Model list includes Anthropic Claude, OpenAI GPT, and DeepV models
- Fallback to a default window size for unknown models

## Related

- [[llm-provider-system]] — Transforms are applied before provider calls
- [[context-compaction]] — Uses context window registry
- [[stream.go]] — Streaming types used by both transform and retry
