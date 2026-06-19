---
type: entity
date: 2026-06-14
tags: [tools, external, http-callback, registration]
---

# External Tool System

> Mechanism for registering and executing tools provided by external processes via HTTP callbacks.

## Source

`internal/agent/external_tool.go` — External tool registration and HTTP callback execution.

## How It Works

External tools are registered via:
- API endpoint `POST /tools/register` (in [[server-websocket]])
- Programmatic API at startup

Each external tool definition includes:
- **Name** — Tool identifier for LLM invocation
- **Description** — Purpose description for LLM context
- **Parameters** — JSON Schema defining expected arguments
- **Callback URL** — HTTP endpoint to invoke for execution

## Execution Flow

```
LLM requests tool → Agent detects external tool
    → HTTP POST to callback URL with parameters
    → External process executes
    → Response returned to agent as ToolResult
```

## Types

The `ExternalToolDef` struct mirrors the agent.Tool interface:

```go
type ExternalToolDef struct {
    Name        string
    Description string
    Parameters  json.RawMessage
    CallbackURL string
}
```

Tool callbacks also support streaming via `PartialResult` to provide real-time progress updates.

## Registration

```go
// In server.go
POST /tools/register → creates ExternalToolDef → adds to agent's tool list
```

## Related

- [[tool-system]] — External tools integrate with the core tool system
- [[server-websocket]] — API endpoint for registration
- [[agent-core]] — Tool execution dispatches to registered external tools
