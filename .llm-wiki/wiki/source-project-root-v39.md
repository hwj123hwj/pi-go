---
type: source
source_path: "."
date: 2026-06-28
tags: [mobile, self-update, version-management, model-selector, music-bar, css, capacitor]
---

# Source: Project Root — v39 Re-ingest

> Mobile self-update polish, manual check-update button, model selector restored, music bar transform fix, version management lesson.

## Key Takeaways

1. **Version bump is mandatory for self-update to work** — APKs uploaded to the same GitHub release tag don't trigger the update dialog because `isNewer(latest, current)` compares the release tag version against the installed APK's `versionName`. When both are `0.6.0`, there's no update to push. Bumped to `0.7.0` (versionCode 7) and created a new GitHub Release.

2. **Manual check-update button added** — The auto-check (3s after launch, silent) gave no user-visible feedback. Added a `↻` refresh icon button in the Sidebar footer (mobile/Capacitor only). Clicking it calls `checkMobileUpdate()` and either shows the dialog (if update available) or alerts "已是最新版本 vX.X.X". Communication with `MobileUpdateDialog` via `window.dispatchEvent(new CustomEvent('pi-go-show-update', { detail: info }))`.

3. **Model selector restored on mobile** — Three changes: (a) Removed `{isElectron && ...}` guard in `PromptBar.tsx`; (b) Removed same guard in `SessionView.tsx`; (c) Changed mobile CSS `.prompt-config { display: none }` → `display: flex` with compact sizing (12px font, max-width 200px).

4. **Music bar `transform: none` fix** — Desktop `.global-music-bar` uses `transform: translateX(-50%)` for bottom-center positioning. Mobile override set `left: 0; right: 0; width: 100%` but forgot to clear the transform, causing the bar to be shifted half its width to the left. Fixed: added `transform: none !important`.

## Important Entities

- [[desktop-app]] — Mobile self-update, version management, sidebar footer button
- [[music-agent]] — Music bar CSS fix (transform conflict)
- Mobile update flow: `mobile-updater.ts` → GitHub Releases API → semver comparison

## Notable Claims

- **Semver comparison**: `checkMobileUpdate()` fetches GitHub's latest release, extracts the tag name (e.g., `v0.7.0`), strips the `v` prefix, and compares against `getAppVersion()` (Capacitor App plugin). If `latest > current`, returns `MobileUpdateInfo`.
- **VersionCode vs VersionName**: Android `versionCode` is an integer (must increment), `versionName` is the display string (e.g., `0.7.0`). Both bumped: 6→7, `0.6.0`→`0.7.0`.
- **Custom event bridge**: Sidebar button dispatches `pi-go-show-update` CustomEvent; `MobileUpdateDialog` listens for it in its `useEffect`. This decouples the trigger (sidebar) from the UI (dialog) without shared state.

## Files Changed

| File | Change |
|------|--------|
| `desktop/src/components/Sidebar.tsx` | Added `Capacitor.isNativePlatform()` check-update ↻ button in footer |
| `desktop/src/components/MobileUpdateDialog.tsx` | Added `pi-go-show-update` event listener for manual trigger |
| `desktop/src/components/PromptBar.tsx` | Removed `isElectron` guard on model selector |
| `desktop/src/components/SessionView.tsx` | Removed `isElectron` guard on model selector |
| `desktop/src/styles/app.css` | `.prompt-config` mobile: `display: none` → `display: flex` (compact); `.global-music-bar` mobile: added `transform: none !important` |
| `desktop/android/app/build.gradle` | `versionCode 6→7`, `versionName "0.6.0"→"0.7.0"` |

## Cross-References

- [[desktop-app]] — Mobile self-update system (v38), mobile optimizations table
- [[source-project-root-v38]] — Original self-update implementation
