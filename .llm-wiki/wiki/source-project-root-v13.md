---
type: source
source_path: .
date: 2026-06-25
tags: [desktop, file-editor, workspace-write-api, inline-editing]
---

# Source: Project Root (.) — v13: Workspace Inline File Editor

## Key Takeaways

1. **Workspace Files Now Editable Inline**: The FilesPanel (right sidebar file browser) previously only displayed files in read-only mode (Markdown preview or syntax-highlighted code). Users now have a full inline editing workflow:
   - **Edit button**: appears in the file content toolbar for all text files
   - **Textarea editor**: full-size textarea replaces the code preview when editing
   - **Unsaved indicator**: a pulsing amber dot + "Unsaved" text appears when content diverges from the original
   - **Save/Cancel buttons**: save writes to disk via the new backend API; cancel reverts to read-only view
   - **Save disabled until dirty**: the save button is disabled unless the user has actually changed content
   - **Markdown toggle hidden during edit**: when editing a Markdown file, the preview/source toggle is hidden to maximize editing space

2. **New Backend Endpoint**: `PUT /workspace/write-file?path=...` — writes file content to disk. This is a workspace-scoped variant of the existing `PUT /sessions/{id}/file` endpoint. Both share the same request body (`{ content: string }`) and behavior.

## Architecture

```
┌──────────────────────────────────────────────┐
│ FilesPanel (right sidebar)                   │
│                                               │
│  ┌─ file-content-toolbar ─────────────────┐  │
│  │  [Preview] [Source]    [Edit]          │  │
│  └────────────────────────────────────────┘  │
│                                               │
│  ┌─ file content area ────────────────────┐  │
│  │  HighlightedCode (read-only)            │  │
│  │  —or—                                    │  │
│  │  Markdown (rendered)                     │  │
│  │  —or (editing)—                          │  │
│  │  textarea (full edit)                    │  │
│  └────────────────────────────────────────┘  │
│                                               │
│  Edit Mode:                                   │
│  ┌─ file-content-toolbar ─────────────────┐  │
│  │  ● Unsaved          [Cancel] [Save]    │  │
│  └────────────────────────────────────────┘  │
│  ┌─ file-edit-area ───────────────────────┐  │
│  │  <textarea>                             │  │
│  │  ...                                    │  │
│  └────────────────────────────────────────┘  │
└──────────────────────────────────────────────┘
```

## Code Changes

### Backend: `internal/server/server.go`
- **Route**: `PUT /workspace/write-file`
- **Handler**: `Server.workspaceWriteFile`
- **Request**: `{ "content": "..." }`
- **Response**: `{ "status": "ok" }`
- **Behavior**: creates parent directories if needed, writes file with 0644 permissions

### Frontend: `desktop/src/components/workspace/FilesPanel.tsx`
- **`writeFileText(path, content)`**: new REST helper → calls `PUT /workspace/write-file`
- **`FileContent` component**: added `editing`, `editContent`, `saving`, `dirty` state
  - `handleStartEdit()`: copies loaded text into edit buffer
  - `handleCancelEdit()`: discards changes, returns to read-only
  - `handleSave()`: writes to disk, updates loaded content on success
  - Toolbar now shows Edit/Save/Cancel buttons + unsaved indicator
  - When editing, renders `<textarea class="file-edit-area">` instead of `HighlightedCode`

### Styling: `desktop/src/styles/app.css`
- `.file-content-toolbar`: now flex with gap, min-height 30px
- `.file-edit-btn`: bordered chip buttons with hover states
- `.file-edit-btn.save`: accent-colored with white-on-hover
- `.file-edit-btn.cancel`: danger-colored on hover
- `.file-dirty-indicator`: amber dot + text, opacity-toggled
- `.file-edit-area`: full-size textarea with mono font

### i18n: `zh.ts` + `en.ts`
- `files.edit`: 编辑 / Edit
- `files.save`: 保存 / Save
- `files.cancel`: 取消 / Cancel
- `files.unsaved`: 未保存 / Unsaved

## Related Pages

- [[desktop-app]] — Desktop application architecture
- [[source-project-root-v12]] — Previous desktop fixes

## Contradictions

None — new feature addition.
