---
type: source
source_path: .
date: 2026-06-25
tags: [desktop, react, chat-scroll, kb-panel, entry-loading, tag-view, smart-scroll]
---

# Source: Project Root (.) — v12: Desktop Chat Smart Scroll & KB Tag View Loading

## Key Takeaways

1. **Chat Auto-Scroll Hijacks User Scroll Position**: `ChatPane` unconditionally called `bottomRef.scrollIntoView()` on every transcript update, even when the user had scrolled up to read earlier messages. This made it impossible to read history while the agent was responding — every new token would yank the view back to the bottom. Fixed with a "sticky to bottom" pattern:
   - Added `scrollContainerRef` on the scrollable `.pane-body` element
   - Added a `stickToBottomRef` (mutable ref, not state, to avoid re-renders) that tracks whether the user is within 80px of the bottom
   - `scroll` listener updates the ref on every scroll event (passive)
   - Auto-scroll only fires when `stickToBottomRef.current === true`
   - This is the standard pattern used by Slack, Discord, and most chat UIs

2. **KB Tag View Empty List While Loading**: The tag detail view (when a specific tag is selected) rendered `entries.map(...)` directly with no loading state. When switching tags, the old entries would briefly show before the new fetch resolved. Fixed by adding an `entries.length === 0` check that shows a spinner instead of an empty list.

3. **KB Tag View Back Button Accessibility**: The back button in tag detail view had no `title`/tooltip. Added `t('common.back')` as title.

## Code Changes

### ChatPane.tsx — Smart Scroll
```typescript
// Before: always scroll to bottom
useEffect(() => {
  bottomRef.current?.scrollIntoView({ behavior: 'auto' });
}, [view.transcript, showDots]);

// After: only scroll if user is already at the bottom
const scrollContainerRef = useRef<HTMLDivElement>(null);
const stickToBottomRef = useRef(true);

useEffect(() => {
  const el = scrollContainerRef.current;
  if (!el) return;
  const onScroll = () => {
    const threshold = 80;
    stickToBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < threshold;
  };
  el.addEventListener('scroll', onScroll, { passive: true });
  return () => el.removeEventListener('scroll', onScroll);
}, []);

useEffect(() => {
  if (stickToBottomRef.current) {
    bottomRef.current?.scrollIntoView({ behavior: 'auto' });
  }
}, [view.transcript, showDots]);
```

### KbPanel.tsx — Tag View Loading State
```tsx
// Before: render entries directly, no loading state
{entries.map((e) => ( <button>...</button> ))}

// After: show spinner when entries is empty (during fetch)
{entries.length === 0 ? (
  <div className="empty"><Icon name="loader" size={16} spin /></div>
) : (
  entries.map((e) => ( <button>...</button> ))
)}
```

## Related Pages

- [[desktop-app]] — Desktop application architecture

## Contradictions

None — UX polish release.
