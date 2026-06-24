---
type: source
source_path: .
date: 2026-06-25
tags: [desktop, i18n, react, sidebar, chatpane, menu-dismiss]
---

# Source: Project Root (.) — v11: Desktop i18n Gaps, React Subscription Bug, Menu Dismiss

## Key Takeaways

1. **Hardcoded English Strings (i18n Gap)**: Three strings in the desktop UI were hardcoded in English instead of using the i18n system:
   - `FilesPanel.tsx` CodeContextMenu: "Search with Google", "Copy", "Select All"
   - `Sidebar.tsx`: "Choose mode" (new-session mode dropdown button title)

   All replaced with `t()` calls and added corresponding keys (`codeMenu.searchGoogle`, `codeMenu.copy`, `codeMenu.selectAll`, `mode.chooseMode`) to both `zh.ts` and `en.ts`.

2. **React Non-Reactive Store Read (ChatPane)**: `ChatItemView` used `useStore.getState().sessions[sessionId]?.meta.cwd` — a one-time snapshot read that would NOT re-render when the session's cwd changed (e.g., during session restoration). Fixed by switching to the reactive `useStore((s) => s.sessions[sessionId]?.meta.cwd)` selector, which is the standard Zustand pattern for subscribing to store slices.

3. **Sidebar New-Session Menu Stuck Open**: The new-session mode dropdown (`showNewMenu`) in `Sidebar.tsx` had no outside-click or Escape-to-close handler. Once opened, the only way to close it was to click the toggle button again or select an option. This was inconsistent with other dropdown menus in the app (FileToolbar, EmptyState project selector) which all have outside-click dismissal. Added a `useEffect` listener with `.new-session-wrap` containment check + Escape key handler.

## Code Changes

### zh.ts + en.ts — New i18n Keys
```typescript
// zh.ts
'codeMenu.searchGoogle': '用 Google 搜索',
'codeMenu.copy': '复制',
'codeMenu.selectAll': '全选',
'mode.chooseMode': '选择模式',

// en.ts
'codeMenu.searchGoogle': 'Search with Google',
'codeMenu.copy': 'Copy',
'codeMenu.selectAll': 'Select All',
'mode.chooseMode': 'Choose mode',
```

### ChatPane.tsx — Reactive Store Subscription
```typescript
// Before: non-reactive snapshot
const cwd = useStore.getState().sessions[sessionId]?.meta.cwd;

// After: reactive subscription
const cwd = useStore((s) => s.sessions[sessionId]?.meta.cwd);
```

### Sidebar.tsx — Menu Dismiss Handler
```typescript
useEffect(() => {
  if (!showNewMenu) return;
  const onDown = (e: MouseEvent) => {
    const target = e.target as HTMLElement;
    if (!target.closest('.new-session-wrap')) setShowNewMenu(false);
  };
  const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setShowNewMenu(false);
  document.addEventListener('mousedown', onDown);
  document.addEventListener('keydown', onKey);
  return () => { /* cleanup */ };
}, [showNewMenu]);
```

## Related Pages

- [[desktop-app]] — Desktop application architecture

## Contradictions

None — bugfix/polish release, no architectural changes.
