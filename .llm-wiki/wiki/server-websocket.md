---
type: entity
date: 2026-06-14
tags: [server, http, api, websocket, sse, rest]
---

# Server & WebSocket

> HTTP server providing REST API + Server-Sent Events (SSE) + WebSocket for agent interaction.

## Entry Points

- `cmd/pi-agent/main.go` — Server mode (`-mode serve`)
- `internal/mode/serve.go` — Server startup logic
- `internal/server/server.go` — Route definitions and handlers
- `internal/server/websocket.go` — WebSocket handler

## Architecture

```
Client (Desktop/CLI/Feishu)
    ↓ HTTP/REST + SSE/WebSocket
server.Server
    ├── REST: session CRUD, model/tool listing, compact, command
    ├── SSE: POST /chat/stream → streaming text_delta events
    └── WebSocket: Full-duplex agent communication
        ↓
runtime.SessionRegistry
    ↓
AgentSession
```

## REST API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/health` | Health check → `{"status":"ok"}` |
| `POST` | `/chat` | Synchronous chat → full response |
| `POST` | `/chat/stream` | SSE streaming chat → text delta events |
| `GET` | `/sessions` | List all sessions |
| `POST` | `/sessions` | Create session (accepts `cwd`, `model`) |
| `GET` | `/sessions/{id}/messages` | Get session messages |
| `GET` | `/sessions/{id}/info` | Get session metadata |
| `DELETE` | `/sessions/{id}` | Delete a session |
| `POST` | `/sessions/{id}/model` | Switch model |
| `POST` | `/sessions/{id}/compact` | Compact context |
| `POST` | `/sessions/{id}/command` | Execute slash command |
| `GET` | `/sessions/{id}/diff` | Git diff for session workspace |
| `GET` | `/sessions/{id}/file` | Read file from session workspace |
| `GET` | `/models` | List available models (fetched dynamically from [[config-system]]) |
| `GET` | `/tools` | List registered tools |
| `POST` | `/tools/register` | Register external tool |
| `GET` | `/ws` | WebSocket connection |

## Response Types

| Type | Fields |
|------|--------|
| `ChatResponse` | `text`, `tool_calls`, `session_id` |
| `SessionResponse` | `id`, `created_at`, `message_count` |
| `ErrorResponse` | `error` string |

## Middleware Chain

```
corsMiddleware → recoveryMiddleware → loggingMiddleware → handler
```

- **CORS**: Permissive for desktop app (`Access-Control-Allow-Origin: *`)
- **Recovery**: Catches panics per handler, returns 500
- **Logging**: Method + path + duration + status code

## WebSocket (`/ws`)

The WebSocket handler (`websocket.go`) provides:
- Full-duplex communication with the agent
- Real-time transcript streaming
- Session management commands
- Tool execution updates

## Event Streaming (SSE)

The `POST /chat/stream` endpoint streams events in SSE format:

| Event Type | Description |
|------------|-------------|
| `text_delta` | Incremental text token |
| `tool_start` | Tool execution started |
| `tool_update` | Tool progress update |
| `tool_end` | Tool execution completed |
| `error` | Error occurred |
| `done` | Response complete |

## Session Registry

`internal/runtime/session_registry.go` provides thread-safe session management:
- `GetOrCreate(id, factory)` — Session lookup or creation
- `Get(id)` — Session lookup
- `Delete(id)` — Session termination
- Uses `sync.RWMutex` for concurrent access

## Route Hierarchy

The top-level mux separates REST, WebSocket, and Web UI:

```
GET /ws          → WebSocket handler
/health, /chat, /sessions/*, /models, /tools → REST middleware chain
/*               → Web UI static files ([[web-embed]])
```

## Related

- [[desktop-app]] — Primary consumer of the server API
- [[feishu-integration]] — Alternative consumer via bridge
- [[web-embed]] — Embedded SPA served at root path
- [[runtime-application-interface]] — Sessions are created via Application
