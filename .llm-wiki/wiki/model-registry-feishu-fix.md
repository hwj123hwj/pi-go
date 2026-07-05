# Model Registry & Feishu Multi-Chat Fix

> Source: `internal/models/`, `internal/feishu/handler.go`, `internal/feishu/tool.go`

## Overview

Two improvements in this batch:

1. **Config-driven Model Registry** — replaces hardcoded model lists with a flexible JSON-configurable registry
2. **Feishu Multi-Chat Sender Race Fix** — fixes a concurrency bug where multiple chats would overwrite each other's sender context

---

## 1. Model Registry

### Problem

`CodingApplication.AvailableModels()` returned a hardcoded list of 5 models. The roadmap explicitly flagged this as a weakness: "模型元数据注册表较薄". Adding new models required code changes.

### Solution

`internal/models/registry.go` provides:

```
┌──────────────────────────────────────────────────────────┐
│                    Model Registry                        │
│                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │ Built-in     │  │ JSON Config  │  │ Programmatic │   │
│  │ Defaults(13) │  │ (user file)  │  │  Register()  │   │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘   │
│         │                 │                 │            │
│         └────────┬────────┴────────┬────────┘           │
│                  ▼                 ▼                     │
│           ┌─────────────────────────────┐               │
│           │    Registry (merged)         │               │
│           │  Register / Get / List      │               │
│           │  ListByProvider / Default   │               │
│           └──────────────┬──────────────┘               │
│                          │                              │
│                          ▼                              │
│           CodingApplication.AvailableModels()            │
└──────────────────────────────────────────────────────────┘
```

### Config File Resolution

Priority order:
1. `PI_GO_MODELS_FILE` env var
2. `~/.pi-go/models.json`
3. `<data-dir>/models.json`

### Built-in Default Models (13)

| Provider | Models |
|----------|--------|
| Anthropic | claude-sonnet-4-6, claude-sonnet-4-5, claude-sonnet-4, claude-opus-4, claude-haiku-4 |
| OpenAI | gpt-4o, gpt-4o-mini, gpt-5, gpt-5-mini |
| OpenAI-compatible | deepseek-chat, deepseek-coder, qwen-max, qwen-plus |

### User Config Example (`~/.pi-go/models.json`)

```json
[
  {
    "id": "glm-opus",
    "provider": "openai",
    "name": "GLM Opus",
    "context_window": 128000,
    "max_tokens": 4096
  }
]
```

---

## 2. Feishu Multi-Chat Sender Race Fix

### Problem

```go
// BEFORE (BUG): single shared field — concurrent chats overwrite each other
type Handler struct {
    lastSenderOpenID string  // ← race condition!
    senderMu         sync.Mutex
}
```

When user A in chat X sends a message, then user B in chat Y sends a message before the tool callback fires for chat X, `lastSenderOpenID` gets overwritten to user B. The tool callback for chat X would then use user B's OpenID — wrong person gets invited to the group.

### Solution

```go
// AFTER (FIXED): per-chat map
type Handler struct {
    senders  map[string]string  // chatKey → senderOpenID
    senderMu sync.Mutex
}

func (h *Handler) storeSender(chatKey, senderOpenID string) { ... }
func (h *Handler) getSender(ctx, chatKey string) string { ... }
```

### Tool Callback Enhancement

`ToolCallbackRequest` now includes `session_id` so the tool callback can trace back to the originating chat:

```go
type ToolCallbackRequest struct {
    ToolName  string          `json:"tool_name"`
    Params    json.RawMessage `json:"params"`
    SessionID string          `json:"session_id,omitempty"` // NEW
}
```

The tool callback uses `session_id` as the chatKey to look up the correct sender. Falls back to `getAnySender()` if the chat context is unknown.

---

## Test Results

| Package | Tests | Status |
|---------|-------|--------|
| `internal/models/` | 11 | ✅ All pass |
| `internal/feishu/` | Existing | ✅ All pass |
| Full suite | 30+ packages | ✅ All pass |
