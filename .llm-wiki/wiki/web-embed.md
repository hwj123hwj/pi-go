---
type: entity
date: 2026-06-14
tags: [web, ui, static-files, spa, embed]
---

# Web UI (Embedded SPA)

> Static file server for the embedded Single Page Application (Web UI) served at the root path.

## Source

`internal/web/` — Two files:

| File | Purpose |
|------|---------|
| `handler.go` | HTTP handler for SPA static file serving with fallback |
| `embed.go` | Embedding static files via `//go:embed` directive |

## How It Works

- Static files are embedded into the Go binary using `//go:embed static/*`
- Served at the root path `/*` in the [[server-websocket|top-level mux]]
- Uses SPA fallback: unrecognized paths serve `index.html`
- The static files are built from the React frontend (via Vite) and copied to `internal/web/static/`

## Route Integration

The Web UI is the catch-all route in the server:

```
GET /ws            → WebSocket handler
/health, /chat, …  → REST API middleware chain
/*                  → Web UI (static files + SPA fallback)
```

## Related

- [[server-websocket]] — Server route hierarchy
- [[desktop-app]] — The desktop app is the primary frontend; Web UI is an alternative
