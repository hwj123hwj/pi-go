---
type: entity
date: 2026-06-14
tags: [feishu, lark, bridge, bot, messaging]
---

# Feishu (Lark) Integration

> Bridge service that connects pi-go agent to Feishu/Lark group chats, enabling natural language interaction via the Feishu messaging platform.

## Entrypoint

`cmd/pi-feishu-bridge/main.go` — Standalone binary that starts the Feishu gateway alongside the pi-go agent server.

## Architecture

```
Feishu Client App
    ↓ (WebSocket Event)
pi-feishu-bridge (gateway + handler)
    ↓ (HTTP/SSE)
pi-go Agent Server
    ↓
AgentSession
```

## Internal Package: `internal/feishu/`

The package contains 7 files:

| File | Purpose |
|------|---------|
| `client.go` | Feishu OpenAPI SDK wrapper — sends messages, fetches user info |
| `gateway.go` | WebSocket gateway — maintains persistent connection to Feishu event endpoint |
| `handler.go` | Chat message routing — maps chat → agent session, manages session lifecycle |
| `markdown.go` | Markdown → Feishu Post Content JSON converter (recursive node traversal) |
| `markdown_style.go` | Markdown style optimizations for Feishu CardKit display |
| `cardkit.go` | Feishu CardKit 2.0 interactive card builder — streaming status, metrics display |
| `tool.go` | Tool callback request/response types (mirrors `agent.ExternalToolDef`) |

## Key Components

### WebSocket Gateway (`gateway.go`)
- Maintains a persistent WebSocket connection to Feishu's event endpoint
- Handles reconnection on disconnect
- Routes events to the handler

### Chat Handler (`handler.go`)
- Maps Feishu chat IDs to pi-go session IDs
- Manages agent session lifecycle per chat
- Routes user messages to the appropriate agent session
- Supports multiple simultaneous group chats

### Markdown Converter (`markdown.go`)
- Converts Markdown content to Feishu's structured Post Content JSON format
- Handles: headings, bold, italic, inline code, code blocks, links, lists, blockquotes
- Recursive document tree traversal

### CardKit Builder (`cardkit.go`)
- Builds Feishu CardKit 2.0 interactive cards
- Shows agent streaming status (thinking/typing)
- Displays tool execution metrics
- Renders final responses as rich cards

### Tool Callbacks (`tool.go`)
- Mirrors the `agent.ExternalToolDef` types
- Enables Feishu messages to trigger tool execution
- Supports tool result rendering back to chat

## Deployment

Systemd service: `deploy/pi-feishu-bridge.service`

```bash
# Build
go build -o pi-feishu-bridge ./cmd/pi-feishu-bridge

# Run with systemd
sudo systemctl start pi-feishu-bridge
```

## Configuration

Requires Feishu app credentials (App ID, App Secret) configured via environment variables.

## Related

- [[server-websocket]] — The pi-go server that bridges communicate with
- [[runtime-application-interface]] — Agent sessions are created via Application interface
- [[source-project-root]] — `cmd/pi-feishu-bridge/main.go`
