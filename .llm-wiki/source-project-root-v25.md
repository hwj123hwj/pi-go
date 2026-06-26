# Source: Project Root (.) — v25: Desktop profile panel — user profile UI adaptation

> Date: 2026-06-27
> Focus: After adding the unified user profile + memory extraction + KB vector search, the desktop frontend needed adaptation to surface these features.

## Summary

The backend gained several powerful features (user profile, memory extraction, KB vector search, tool synopsis) but the desktop UI had no way to visualize the user profile. This change adds a **Profile Panel** to the right sidebar, with supporting REST API endpoints.

## Changes

### Backend: Profile REST API (`/profile`)

**`internal/profile/store.go`** — Added `AllFacts()` method:
- Returns all facts across all categories, sorted by recency within each category
- Thread-safe (locks mutex)
- Used by the REST handler to serialize the full profile state

**`internal/app/app.go`** — Wired `profile.Store` into `App`:
- New `Profile` field on `App` struct + `AppOptions`
- New `Profile()` getter on `App` for server access
- Injected from `cmd/pi-agent/main.go`

**`internal/server/profile_handler.go`** (NEW) — REST endpoints:
| Endpoint | Method | Description |
|---|---|---|
| `/profile` | GET | Returns all categories + facts + summary string |
| `/profile` | DELETE | Deletes a specific fact by category+key |

Response format:
```json
{
  "categories": [
    {
      "name": "coding",
      "label": "开发",
      "count": 5,
      "facts": [{ "key": "language", "value": "Go", "source": "coding-agent", ... }]
    }
  ],
  "summary": "## 用户画像\n- 开发：language: Go...",
  "total_facts": 12
}
```

**`internal/server/server.go`** — Registered `/profile` route in `topMux`.

### Frontend: Profile Panel

**`desktop/src/components/workspace/ProfilePanel.tsx`** (NEW):
- Fetches `/profile` and displays all facts grouped by category
- Shows the "Agent Summary" — the exact string injected into agent prompts (with Markdown rendering)
- Expandable/collapsible category sections with icons (code/music/user)
- Per-fact metadata: source agent, last-updated time, access count
- Delete button (hover-reveal) for individual facts
- Empty state with helpful hint

**`desktop/src/components/workspace/RightSidebar.tsx`**:
- Added `profile` to `RightView` type and rail items
- Renders `<ProfilePanel />` when `rightView === 'profile'`

**`desktop/src/store.ts`**:
- Added `'profile'` to `RightView` union type

**`desktop/src/components/Icon.tsx`**:
- Added `user` icon (person silhouette)

**`desktop/src/i18n/locales/zh.ts` + `en.ts`**:
- Added `workspace.profile`, `profile.title`, `profile.facts`, `profile.empty`, `profile.loadFailed`, `profile.agentSummary`, `profile.delete`, `profile.hint`

**`desktop/src/styles/app.css`**:
- Full styling for `.profile-panel`, `.profile-header`, `.profile-summary`, `.profile-categories`, `.profile-cat-*`, `.profile-fact-*` (hover-reveal delete, category icons, summary card with accent border)

## Architecture

```
cmd/pi-agent/main.go
  └─ app.New(Profile: userProfile)        ← profile.Store injected into App
       └─ server → /profile GET/DELETE    ← REST endpoints
            └─ ProfilePanel.tsx           ← Desktop UI
```

The profile store is **shared** — the same instance is used by:
1. KB agent (injected into system prompt via `Summary()`)
2. Music agent (injected via `SummaryForCategories(music, general)`)
3. Desktop UI (read/written via REST API)

## Verification

```
go build ./...     # clean
go vet ./...       # clean
go test -race      # all pass
npx tsc --noEmit   # clean
npm run build      # clean
```

## Cross-references
- [[source-project-root-v24]] — Code review round 3 (where stale removal persistence was fixed)
- [[personal-assistant-roadmap]] — Roadmap for personal assistant features
- [[config-system]] — Config for KBEmbeddingAPIKey etc.
