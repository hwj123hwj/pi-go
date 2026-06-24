---
type: source
source_path: .
date: 2026-06-25
tags: [kb-panel, route-fix, null-guard, desktop, server]
---

# Source: Project Root (.) — v8: KB Panel Crash Fix

## Key Takeaways

1. **KB Route Registration Bug**: The `/kb/*` endpoints were registered on `restMux` but NOT on `topMux`, causing all KB API requests to be caught by the web UI catch-all route and returning HTML instead of JSON.

2. **Null Tags Guard**: The `entryToJSON()` function in `kb_handler.go` directly assigned `e.Tags` to the JSON response. When Go slices are `nil`, JSON serialization outputs `null` instead of `[]`, causing frontend `TypeError: Cannot read property 'length' of null`.

3. **Desktop KB Panel**: New `KbPanel.tsx` component with three views (Browse, Tags, Health) and full i18n support for Chinese/English.

4. **Backend KB Handler**: New `kb_handler.go` with 6 REST endpoints for knowledge base browsing (stats, entries, categories, tags, health, read).

5. **806 Knowledge Cards**: The KB now indexes 806 entries across 11 categories and 2028 tags from the agent-lessons repository.

## Code Changes

### server.go — Route Registration Fix
```go
// Added /kb/ route to topMux (was missing)
topMux.Handle("/kb/", restHandler)
```

### kb_handler.go — Null Tags Guard
```go
func entryToJSON(e kbtools.Entry) kbEntryJSON {
    tags := e.Tags
    if tags == nil {
        tags = []string{}  // Ensure JSON outputs [] instead of null
    }
    return kbEntryJSON{
        // ... other fields ...
        Tags: tags,
    }
}
```

## Related Pages

- [[server-websocket]] — HTTP REST + SSE + WebSocket server
- [[kb-agent]] — Knowledge base retrieval agent
- [[desktop-app]] — Electron + React GUI client

## Contradictions

None — this is a bugfix release, no architectural changes.
