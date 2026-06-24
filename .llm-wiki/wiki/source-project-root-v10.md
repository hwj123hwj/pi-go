---
type: source
source_path: .
date: 2026-06-25
tags: [desktop, react, race-condition, websocket, memory-leak, abort, files-panel, kb-panel]
---

# Source: Project Root (.) — v10: Desktop Race Condition & Memory Leak Fixes

## Key Takeaways

1. **React State Update After Unmount (FilesPanel)**: `FileTree` and `TreeNode` components in FilesPanel made async fetch calls (`searchFiles`, `listDir`) without Abort/cleanup guards. If the user switched sessions or navigated away before the promise resolved, React would attempt to update state on an unmounted component, causing memory leaks and console warnings. Fixed by adding `let alive = true` guards with cleanup functions in all `useEffect` hooks.

2. **React State Update After Unmount (KbPanel)**: Several `useEffect` hooks in KbPanel (`fetchCategories`, `fetchEntries`, `fetchTags`, `fetchEntries-for-tag`) used `.then(setState)` without cleanup guards, risking the same unmount race condition. Fixed with `alive` flag pattern in all data-fetching effects.

3. **WebSocket Reconnect Storm**: The `WSService` auto-reconnect used a fixed 2-second delay. When the backend was down for an extended period, the client would fire reconnect attempts every 2 seconds indefinitely, creating unnecessary network load. Fixed with exponential backoff (2s → 4s → 8s → ... → max 30s), with the backoff counter reset on successful connection and the reconnect timer cleared on explicit disconnect.

4. **FileToolbar Empty Dir Edge Case**: `file.replace(/[\\/][^\\/]*$/, '')` could return an empty string when the file path had no directory separator, causing the "Open in Terminal" button to receive an invalid path. Fixed with `|| file` fallback.

5. **Debounce Timer Race**: The KbPanel Browse view's 300ms debounce `setTimeout` created a new closure on every keystroke. While the outer `clearTimeout` correctly cancelled the timer, the fetch promise inside could still resolve and call `setEntries` on stale data. Fixed by adding `alive` flag inside the timeout callback.

## Code Changes

### FilesPanel.tsx — Abort Guards
```typescript
// Before: no cleanup → unmount race
useEffect(() => {
  void searchFiles(root)
    .then((list) => setFiles(list))
    .catch(() => setFiles([]));
}, [...]);

// After: alive flag + cleanup
useEffect(() => {
  let alive = true;
  void searchFiles(root)
    .then((list) => alive && setFiles(list))
    .catch(() => alive && setFiles([]))
    .finally(() => alive && setLoading(false));
  return () => { alive = false; };
}, [...]);
```

### store.ts — WebSocket Exponential Backoff
```typescript
class WSService {
  private reconnectAttempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  connect(url) {
    // ...
    this.ws.onopen = () => {
      this.reconnectAttempts = 0;  // reset on success
    };
    this.ws.onclose = () => {
      const delay = Math.min(2000 * Math.pow(2, this.reconnectAttempts), 30000);
      this.reconnectAttempts++;
      this.reconnectTimer = setTimeout(() => this.connect(url), delay);
    };
  }

  disconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    // ...
  }
}
```

### FilesPanel.tsx — Empty Dir Fallback
```typescript
// Before: could return empty string for root-level files
const dir = file.replace(/[\\/][^\\/]*$/, '');
// After: fallback to the file path itself
const dir = file.replace(/[\\/][^\\/]*$/, '') || file;
```

## Related Pages

- [[desktop-app]] — Desktop application architecture
- [[server-websocket]] — WebSocket protocol

## Contradictions

None — bugfix release, no architectural changes.
