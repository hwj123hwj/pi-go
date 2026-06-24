---
type: entity
date: 2026-06-22
tags: [server, http, api, websocket, sse, rest, cors, file-endpoints, gateway]
related: [[desktop-app]], [[feishu-integration]], [[web-embed]], [[runtime-application-interface]], [[session-manager]]
---

# Server & WebSocket

> HTTP server providing REST API + Server-Sent Events (SSE) + WebSocket for agent interaction.

## Entry Points

- `cmd/pi-agent/main.go` — Server mode (`-mode serve`)
- `internal/mode/serve.go` — Server startup logic (thin wrapper)
- `internal/server/server.go` — Route definitions and handlers (1001 lines)
- `internal/server/websocket.go` — WebSocket handler

## Architecture

```
Client (Desktop/CLI/Feishu)
    ↓ HTTP/REST + SSE/WebSocket
server.Server
    ├── REST: session CRUD, model/tool listing, compact, command, file I/O
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
| `POST` | `/sessions` | Create session (accepts `cwd`, `model`, `application`) |
| `GET` | `/sessions/{id}/messages` | Get session messages (includes tool_calls, thinking) |
| `GET` | `/sessions/{id}/info` | Get session metadata (provider, model, workspace) |
| `DELETE` | `/sessions/{id}` | Delete a session |
| `POST` | `/sessions/{id}/model` | Switch model (accepts `model`, optional `provider`) |
| `POST` | `/sessions/{id}/compact` | Compact context (accepts `custom_instructions`) |
| `POST` | `/sessions/{id}/command` | Execute slash command |
| `GET` | `/sessions/{id}/diff` | Git diff for session workspace |
| `GET` | `/sessions/{id}/file?path=` | Read file content from filesystem |
| `PUT` | `/sessions/{id}/file?path=` | Write file content to filesystem |
| `GET` | `/models` | List available models (dynamic from gateway) |
| `GET` | `/tools` | List registered tools |
| `GET` | `/applications` | List available applications |
| `POST` | `/tools/register` | Register external tool |
| `GET` | `/ws` | WebSocket connection |
| `GET` | `/kb/stats` | Knowledge base statistics |
| `GET` | `/kb/entries` | List KB entries (with category/tag/query filters) |
| `GET` | `/kb/categories` | List KB categories with counts |
| `GET` | `/kb/tags` | List KB tags with counts |
| `GET` | `/kb/health` | KB health report (missing metadata, duplicates, tag clusters) |
| `GET` | `/kb/read?path=` | Read KB entry content |

## CreateSession Request

```json
{
  "cwd": "/path/to/project",       // optional: workspace directory
  "model": "claude-sonnet-4-6",    // optional: model override
  "application": "coding"           // optional: "coding", "music", etc.
}
```

The `application` field selects which `runtime.Application` to use for the session. Resolved via `App.SessionDepsWithApp(name)`.

## Dynamic Model Discovery

`GET /models` fetches models dynamically:

1. **Gateway query** — If `OpenAIBaseURL` and `OpenAIAPIKey` are configured, queries `{baseURL}/v1/models` (OpenAI-compatible endpoint)
2. **Fallback** — Returns hardcoded list (DeepSeek V4 Flash, GLM-5, Claude Sonnet 4.6)
3. **Current model** — Determined from config (`OpenAIModel` / `DeepVModel` / `AnthropicModel`)

Response format:
```json
{
  "models": [{"id": "...", "provider": "openai", "name": "..."}],
  "current": {"id": "...", "provider": "openai", "name": "..."}
}
```

## File Endpoints

### GET /sessions/{id}/file?path=

Reads any file from the filesystem. Returns `{"content": "..."}`. Used by desktop file pane for viewing files referenced by the agent.

### PUT /sessions/{id}/file?path=

Writes content to a file. Creates parent directories if needed. Request body: `{"content": "..."}`. Used by desktop file pane for saving edits.

## Session Messages Response

`GET /sessions/{id}/messages` returns enriched message entries:

```json
[{
  "role": "user",
  "content": "..."
}, {
  "role": "assistant",
  "content": "...",
  "thinking": "...",
  "tool_calls": [{"id": "...", "name": "bash", "args": "..."}]
}, {
  "role": "tool",
  "content": "...",
  "tool_call_id": "...",
  "is_error": false
}]
```

## WebSocket Protocol (`/ws`)

### Connection

- Endpoint: `GET /ws`
- Uses `gorilla/websocket` with permissive `CheckOrigin` (allows all origins for desktop)
- Read/write buffer: 1024 bytes
- Wrapped in `wsConn` with mutex for thread-safe writes

### Client → Server Messages

```json
{"type": "prompt", "session_id": "...", "prompt": "hello"}
{"type": "cancel", "session_id": "..."}
{"type": "switch_model", "session_id": "...", "model": "claude-sonnet-4-6", "provider": "openai"}
{"type": "ping"}
```

### Server → Client Messages

| Type | Fields | Description |
|------|--------|-------------|
| `session_id` | `session_id` | Confirms/returns session ID (auto-created if needed) |
| `status` | `session_id`, `streaming` | Streaming state (true=start, false=done) |
| `event` | `session_id`, `event` | Agent event (text_delta, tool_start, tool_end, etc.) |
| `model_info` | `session_id`, `provider`, `model` | Model switch confirmation |
| `error` | `session_id`, `message` | Error occurred |
| `pong` | — | Ping response |

### Prompt Handling Flow

1. Client sends `{type: "prompt", session_id, prompt}`
2. Server cancels any existing prompt for this connection
3. Resolves session (load or create)
4. Sends `session_id` confirmation
5. Sends `status{streaming: true}`
6. Calls `sess.PromptStream()` → gets event channel
7. Goroutine forwards events as `{type: "event", session_id, event}` messages
8. On completion: sends `status{streaming: false}`

### Cancellation

Client sends `{type: "cancel"}` → server calls the context's cancel function → stream ends.

## Middleware Chain

```
corsMiddleware → recoveryMiddleware → loggingMiddleware → handler
```

- **CORS**: `Access-Control-Allow-Origin: *`, methods: `GET, POST, PUT, DELETE, OPTIONS`
- **Recovery**: Catches panics per handler, returns 500 JSON error
- **Logging**: Method + path + duration + status code
- **WebSocket**: Bypasses middleware chain (direct handler) to avoid Hijack issues

## Route Hierarchy

```
GET /ws          → WebSocket handler (no middleware)
/health, /chat, /sessions/*, /models, /tools, /applications → REST middleware chain
/*               → Web UI static files ([[web-embed]])
/music/*         → Extra routes (music audio proxy, if set)
```

## External Tool Registration

`POST /tools/register` accepts:
```json
{
  "name": "my_tool",
  "description": "...",
  "parameters": {...},
  "callback_url": "http://localhost:3000/callback"
}
```

Tools are synced to `App.SetExternalTools()` and become available in subsequent sessions.

## Related

- [[desktop-app]] — Primary consumer of the server API
- [[feishu-integration]] — Alternative consumer via bridge
- [[web-embed]] — Embedded SPA served at root path
- [[runtime-application-interface]] — Sessions created via Application
- [[session-manager]] — Session CRUD backed by sessionmgr.Manager
- [[external-tools]] — HTTP callback tool registration
