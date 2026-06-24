---
type: source
source_path: .
date: 2026-06-25
tags: [kb-panel, css-fix, security, debounce, sorting, desktop, server]
---

# Source: Project Root (.) — v9: KB Panel Hardening (CSS, Security, UX)

## Key Takeaways

1. **Undefined CSS Variable**: `--text-muted` was referenced in ~29 KB panel CSS rules but never defined in `:root` (dark), `[data-theme='light']`, or the system-mode media query. The variable fell through to `var(--text-muted, ...)` fallbacks in 4 spots but was bare (no fallback) in 25 other spots, causing those styles to inherit the browser default (`initial`) instead of the intended muted text color. Fixed by defining `--text-muted` in all three theme blocks (dark: `#b3b0a6`, light: `#6a685f`, matching `--text-dim`).

2. **Hardcoded Fallback Colors**: The KB health dashboard used `var(--orange, #f59e0b)` for warning/duplicate counts, but `--orange` is not part of the design token system. Replaced with the existing `--amber` / `--amber-weak` tokens which are defined in all themes.

3. **Path Traversal Vulnerability (Security)**: The `kbRead` handler (`GET /kb/read?path=`) resolved relative paths via simple string concatenation (`repoPath + "/" + path`) without verifying the final path stays inside the KB repo. A crafted `path` parameter with `../` sequences could read arbitrary files. Fixed by resolving to absolute paths via `filepath.Abs` and enforcing a prefix containment check.

4. **Missing Entry Sorting**: The no-query entries endpoint returned entries in filesystem walk order (arbitrary). Added `sort.Slice` to sort by `modified` date descending so the most recent entries appear first.

5. **Search Input Debounce**: The Browse view's search input triggered an API request on every keystroke with no debounce, causing rapid-fire requests. Added a 300ms `setTimeout` debounce.

6. **Tag View Stale Selection**: When switching between tags in the Tags view, the previously selected entry detail panel wasn't cleared, causing a brief mismatch. Added a `useEffect` to reset `selectedEntry` when `activeTag` changes.

## Code Changes

### app.css — CSS Variable Definition
```css
/* Added to :root (dark), [data-theme='light'], and system-mode media query */
--text-muted: #b3b0a6;  /* dark — same as --text-dim */
--text-muted: #6a685f;  /* light — same as --text-dim */
```

### app.css — Token Replacement
```css
/* Before */
.kb-health-card.warn .kb-health-card-value { color: var(--orange, #f59e0b); }
/* After */
.kb-health-card.warn .kb-health-card-value { color: var(--amber); }
```

### kb_handler.go — Path Traversal Fix
```go
// Security: resolve to absolute and verify inside KB repo
absRepo, _ := filepath.Abs(repoPath)
absFile, err := filepath.Abs(fullPath)
if err != nil {
    writeError(w, http.StatusBadRequest, ...)
    return
}
if !strings.HasPrefix(absFile+sep, absRepo+sep) {
    writeError(w, http.StatusForbidden, "path is outside the knowledge base")
    return
}
```

### kb_handler.go — Entry Sorting
```go
sort.Slice(entries, func(i, j int) bool {
    return entries[i].Modified.After(entries[j].Modified)
})
```

### KbPanel.tsx — Search Debounce
```typescript
useEffect(() => {
    const timer = setTimeout(() => {
        void fetchEntries(...).then(...)
    }, 300);
    return () => clearTimeout(timer);
}, [activeCategory, query]);
```

## Related Pages

- [[kb-agent]] — Knowledge base agent (desktop panel section)
- [[desktop-app]] — Desktop application (KB panel, design system)
- [[server-websocket]] — HTTP REST server (KB endpoints)

## Contradictions

None — bugfix/hardening release, no architectural changes.
