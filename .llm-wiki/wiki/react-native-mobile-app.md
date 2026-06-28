# React Native Mobile App

> Mobile client for Pi-Go, rebuilt from Capacitor to React Native + Expo (2026-06-28)

## Why React Native?

The previous Capacitor (WebView) app had insurmountable limitations:
- **Microphone/ASR failure**: getUserMedia silently fails in Capacitor WebView, three layers of fixes (OS permission plugin, WebChromeClient, native MediaRecorder plugin) all failed
- **Audio format mismatch**: WebView produces webm/opus, which SiliconFlow ASR may not accept
- **Native API access**: WebView sandbox blocks direct hardware access

React Native solves these with true native APIs (expo-av Audio.Recording).

## Tech Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Framework | Expo SDK 52 | Dev-client mode, prebuild, EAS-compatible |
| RN version | 0.76.7 | Stable, Hermes-enabled |
| State | Zustand 5 | Atomic selectors, fine-grained subscriptions |
| Storage | expo-secure-store | Encrypted server URL persistence |
| TypeScript | Strict mode | Zero errors |
| Audio | expo-av (pending CMake fix) | Native AAC recording → ASR |

## Architecture

```
mobile/
├── App.tsx                    # Root: screen routing (connect→list→chat)
├── index.ts                   # registerRootComponent
├── app.json                   # Expo config (permissions, plugins)
├── src/
│   ├── types/index.ts         # ChatItem, SessionMeta, ModelInfo
│   ├── api/
│   │   ├── index.ts           # REST + ASR upload (FormData)
│   │   └── ws.ts              # WebSocket client (auto-reconnect)
│   ├── store/index.ts         # Zustand store (sessions, WS events, models)
│   └── screens/
│       ├── ServerConnect.tsx  # First-run server URL config
│       ├── SessionList.tsx    # Home: session cards, pull-to-refresh
│       └── ChatScreen.tsx     # Chat: streaming messages, model selector
```

## RN Best Practices Applied (react-native-best-practices skill)

### Round 1 — Core Performance (CRITICAL/HIGH)

#### 1. Hermes mmap (bundle-hermes-mmap) — CRITICAL
- `expo.useLegacyPackaging=false` in gradle.properties
- Keeps .so files uncompressed → mmap'd at runtime → faster TTI

#### 2. R8 Code Shrinking (bundle-r8-android) — CRITICAL
- `android.enableProguardInReleaseBuilds=true`
- `android.enableShrinkResourcesInReleaseBuilds=true`
- Only affects release builds, shrinks APK significantly

#### 3. React.memo + useCallback (js-profile-react) — CRITICAL
- All message components (UserBubble, AssistantMessage, ToolMessage, etc.) wrapped in `React.memo`
- `renderItem` wrapped in `useCallback` with proper deps
- `keyExtractor` as stable `useCallback`
- Prevents cascading re-renders during streaming text_delta

#### 4. FlatList Optimization (js-lists-flatlist-flashlist) — CRITICAL
- `removeClippedSubviews={true}` — unmount off-screen items
- `maxToRenderPerBatch={8}` (chat), `6` (session list)
- `windowSize={12}` (chat), `10` (session list)
- `initialNumToRender={12}` / `{10}`
- `getItemLayout` provided for fixed-height estimation (chat)

#### 5. Zustand Atomic Selectors (js-atomic-state) — HIGH
- Each store field subscribed independently: `useStore(s => s.sessions)`, `useStore(s => s.sendPrompt)`
- No broad `useStore(s => s)` that triggers re-render on any state change
- `useCallback` wrappers around selector functions for referential stability

### Round 2 — Bundle & Memory (CRITICAL/MEDIUM)

#### 6. Avoid Barrel Exports (bundle-barrel-exports) — CRITICAL
- Split `api/index.ts` into `api/server-url.ts` + `api/rest.ts` (direct file imports)
- Split `types/index.ts` into `types/ChatItem.ts` + `types/ModelInfo.ts` + `types/SessionView.ts`
- All imports now reference specific files: `import { useStore } from '../store'` not `from '../store/index'`
- Eliminates unnecessary module evaluation at startup

#### 7. Tree Shaking (bundle-tree-shaking) — HIGH
- `.env`: `EXPO_UNSTABLE_METRO_OPTIMIZE_GRAPH=1` + `EXPO_UNSTABLE_TREE_SHAKING=1` (Expo SDK 52+ experimental)
- `metro.config.js`: `experimentalImportSupport: true` + `inlineRequires: true`
- Removes unused exported code from dependencies in production builds

#### 8. Memory Leak Prevention (js-memory-leaks) — MEDIUM
- All WebSocket `.on()` return values stored in `wsUnsubs[]` array
- `store.destroy()` method calls all unsub functions + `wsService.disconnect()`
- WebSocket client: `disconnect()` clears reconnect timer + all listeners
- Exponential backoff (3s → 6s → 12s → max 30s) prevents hammering on disconnect

#### 9. WebSocket Reconnect Backoff — MEDIUM
- Old: fixed 3s reconnect (could hammer server during outage)
- New: exponential backoff with 30s cap and attempt counter
- Resets to 0 on successful connection

### Pending Optimizations
- **expo-av CMake error**: `ReactAndroid` target not found with new architecture. Needs investigation — possibly requires `newArchEnabled=true` alignment or expo-audio migration

### Round 4 — Bundle Analysis, 16KB Alignment, Native Navigation (CRITICAL/HIGH)

### Round 3 — Concurrent React, Error Boundaries, React Compiler (HIGH/MEDIUM)

#### 10. Error Boundaries — HIGH
- Created `src/components/ErrorBoundary.tsx` — Class component catching render crashes
- Wraps **all three screens** in App.tsx (ServerConnect, SessionList, ChatScreen)
- Fallback UI: error message + "重试" button calling `onReset` to recover
- Prevents single-screen errors from killing the entire app session

#### 11. Concurrent React: useDeferredValue (js-concurrent-react) — HIGH
- `deferredTranscript = useDeferredValue(transcript)` — transcript updates on every text_delta (high frequency)
- `deferredBusy = useDeferredValue(busy)` — status changes can lag behind input
- FlatList renders from `deferredTranscript` instead of raw `transcript`
- `renderItem` uses `deferredBusy` — React prioritizes user input over background re-renders
- **Effect**: Typing into TextInput stays responsive even during heavy streaming

#### 12. Uncontrolled TextInput (js-uncontrolled-components) — HIGH
- ChatScreen: added `inputRef` + `textRef` alongside React state
- `onChangeText` updates `textRef.current` (instant, no re-render needed)
- `submit()` reads from `textRef.current` instead of state — no stale closure risk
- Submit also calls `inputRef.current?.setNativeProps({ text: '' })` for instant clear
- **Effect**: Zero-flicker typing, no controlled-component round-trip per keystroke

#### 13. View Flattening Guard (native-view-flattening) — MEDIUM
- ChatScreen root: `<SafeArea collapsable={false}>`
- SessionList root: `<SafeArea collapsable={false}>`
- Prevents RN from flattening screen containers into unexpected hierarchy

#### 14. React Compiler (js-react-compiler) — HIGH
- `babel.config.js`: added `['react-compiler', { target: '18' }]` plugin
- `app.json`: added `"experiments": { "reactCompiler": true }`
- `babel-plugin-react-compiler@beta` installed as devDependency

#### 15. Bundle Analysis (bundle-analyze-js) — CRITICAL
- Generated production Hermes bytecode: **859KB** (`.hbc`), 529 modules
- Top modules: ReactNativeRenderer (312KB), ReactFabric (304KB), ScrollView (70KB)
- **All polyfills are RN core** (buffer, whatwg-url, whatwg-fetch, regenerator) — no unnecessary web polyfills
- **Verdict**: Bundle is clean — 100% RN core code, no bloat from third-party libs
- `expo-asset` installed (was missing for metro config)

#### 16. 16KB Page Size Alignment (native-android-16kb-alignment) — CRITICAL
- `zipalign -c -P 16 -v 4` verification: **All 52 `.so` files OK**
- Includes libhermes.so, libc++_shared.so, libexpo-modules-core.so, etc.
- Created `scripts/check-16kb-alignment.sh` — CI-ready verification script
- Run after every release build: `./scripts/check-16kb-alignment.sh path/to/app.apk`

#### 17. Native Navigation Stack (react-native-screens) — HIGH
- Installed `react-native-screens` + `@react-navigation/native-stack`
- Migrated from manual `useState<Screen>` switching to `createNativeStackNavigator`
- `src/navigation/AppNavigator.tsx`: NavigationContainer + native stack
- Screens use `NativeStackScreenProps` (type-safe route params)
- `freezeOnBlur: true` — off-screen screens frozen in native → lower memory
- `animation: 'slide_from_right'` — native slide transitions
- `ServerConnect.navigation.replace('List')` — replaces stack (no back to connect)
- Hardware back button now works natively

#### 18. Polyfill Audit (native-sdks-over-polyfills) — HIGH
- Audited all 21 polyfill modules in production bundle
- **Result**: All polyfills are RN core infrastructure, not app-installed
- `buffer` (48.9KB) — required by RN for binary operations
- `whatwg-url` (64KB) — RN's built-in URL polyfill, not removable
- `whatwg-fetch` (19.4KB) — RN's built-in fetch implementation
- `regenerator-runtime` (24.6KB) — async/await support, bundled by Babel
- `event-target-shim` (22.9KB) — RN's event system core
- **No app-level polyfills found** — no Intl/crypto-js/extra polyfills to remove

### Round 5 — Bugfix Audit (CRITICAL)

After code review of commits b515893e..b07e3714 (Rounds 1-4), found and fixed 5 bugs introduced by optimizations:

#### BUG-1 (CRITICAL): init() guard race condition
- **Problem**: `initialized = true` was set at the TOP of `init()`, before checking if a server URL existed. When `App.tsx` called `init()` on startup (no URL stored → returned early), then `ServerConnect` called `init()` again after user entered URL, the second call was a silent no-op — WS never connected, sessions never loaded.
- **Fix**: Moved `initialized = true` to AFTER URL validation + base URL set. `init()` now returns `Promise<boolean>` so callers know if it succeeded.

#### BUG-2 (CRITICAL): setActive() never called after navigation migration
- **Problem**: In Round 4, `ChatScreen` was migrated to `NativeStackScreenProps` but `setActive(sessionId)` — which loads the historical transcript from the REST API — was never wired up. Opening an existing session showed an empty chat.
- **Fix**: Added `useEffect(() => { setActive(sessionId) }, [sessionId])` in ChatScreen.

#### BUG-3 (MEDIUM): Fixed getItemLayout causing scroll jumps
- **Problem**: `getItemLayout` used a hardcoded `length: 60` for all messages. Chat messages are variable-height (short text vs long code blocks). FlatList used this for scroll offset calculations → blank gaps, misaligned items, jumpy scrolling.
- **Fix**: Removed `getItemLayout` entirely. FlatList now measures items natively for correct positioning. Trade-off: slightly slower first render, but visually correct.

#### BUG-4 (LOW): ServerConnect fire-and-forget init()
- **Problem**: `void useStore.getState().init()` was fire-and-forget, so `navigation.replace('List')` could execute before WS connected or sessions loaded.
- **Fix**: Changed to `await useStore.getState().init()` before navigating.

#### BUG-5 (LOW): destroy() never called
- **Problem**: `store.destroy()` method was created in Round 2 for WS cleanup, but no component ever called it. App backgrounding/unmount left WS connections and listeners open.
- **Fix**: Added `destroy()` call in `App.tsx` cleanup `useEffect`. Also added `set({ ready: false, connected: false })` to destroy for clean state reset.

### Round 6 — Second Bugfix Audit (CRITICAL/HIGH)

Code review of commit 40671910 (Round 5 fixes) found and fixed 5 new bugs:

#### BUG-6 (CRITICAL): destroy() in useEffect cleanup kills WS in React Strict Mode
- **Problem**: Round 5 added `destroy()` to `App.tsx` root `useEffect` cleanup. In React Strict Mode (enabled by React Compiler), effects run mount→unmount→mount. The cleanup `destroy()` killed the WebSocket on the first unmount, and `initPromiseRef` prevented re-init on re-mount → app permanently disconnected.
- **Fix**: Removed `destroy()` from root component cleanup. Root component should never unmount. `destroy()` is now for explicit user-initiated disconnect only.

#### BUG-7 (CRITICAL): init() has no try/catch → REST/WS failures hang forever
- **Problem**: If REST `/sessions` or `/models` endpoints failed, `init()` threw an uncaught error. `ready` never became `true`, so app showed loading spinner forever with no recovery path.
- **Fix**: Wrapped `loadStoredServerUrl()`, `wsService.connect()`, and `refreshSessions()` in individual try/catch blocks. Failures are logged but don't block `ready: true`.

#### BUG-8 (MEDIUM): sendPrompt creates empty assistant placeholder bubble
- **Problem**: `sendPrompt` pushed both a `userItem` and an empty `assistantItem` (text: '') to the transcript. If a `tool_start` event arrived before any `text_delta`, the empty assistant bubble would remain as an orphan bubble showing nothing (or a spinner forever if the response was tool-only).
- **Fix**: Removed the empty `assistantItem` from `sendPrompt`. The `event:text_delta` handler already creates a fresh assistant bubble when the first delta arrives. For tool-only responses, no orphan bubble is left behind.

#### BUG-9 (MEDIUM): ErrorBoundary reset doesn't work — initialRouteName ignored
- **Problem**: When `ErrorBoundary.handleReset` called `setInitialRoute('Connect')`, the `AppNavigator` already existed and `initialRouteName` is only read on first mount. Changing it post-mount had no effect — the navigator kept showing the crashed state.
- **Fix**: Added `navKey` state in `App.tsx`. ErrorBoundary reset now increments `navKey`, forcing `AppNavigator` to fully remount with the new initial route.

#### BUG-10 (LOW): initPromiseRef rejection unhandled
- **Problem**: If `init()` threw (before BUG-7 fix), the promise stored in `initPromiseRef` was rejected with no `.catch()`, causing "unhandled promise rejection" warnings.
- **Fix**: Wrapped `init()` call in try/catch inside the async IIFE. Also fixed at source by BUG-7's try/catch additions.

### Round 7 — Third Bugfix Audit: WebSocket Lifecycle (CRITICAL/HIGH)

Code review of commit 3694d79c found and fixed 3 bugs in the WebSocket lifecycle:

#### BUG-11 (CRITICAL): Server URL change → WS and REST target different servers
- **Problem**: If the user disconnects from server A, enters server B's URL in ServerConnect, and calls `init()`:
  1. `init()` hits the `if (initialized) return true` guard (from a previous successful init) and short-circuits.
  2. WS stays connected to server A, while REST (using the newly-set baseUrl) hits server B.
  3. Messages sent via WS go to the wrong server. Chat is broken.
- **Fix**: ServerConnect now calls `destroy()` (which resets `initialized=false`) before `init()`. This ensures WS and REST both target the new server.

#### BUG-12 (HIGH): ws.ts connect() doesn't close previous connection
- **Problem**: `connect()` created a new WebSocket without closing the old one. If `connect()` was called twice (e.g. `destroy()` → `init()` cycle), the old WebSocket remained open in the background as a zombie, consuming memory and receiving messages with no handlers.
- **Fix**: `connect()` now calls `doDisconnect()` internally if the URL has changed. `doDisconnect()` nullifies all event handlers before calling `close()` on the old socket, preventing stale callbacks from firing.

#### BUG-13 (MEDIUM): disconnect() doesn't prevent stale onclose → reconnect
- **Problem**: `disconnect()` called `ws.close()`, but the old `onclose` handler was still attached. `onclose` fired asynchronously, calling `scheduleReconnect()`, which created a new WebSocket even though the user explicitly disconnected.
- **Fix**:
  1. Added `connected` flag: `connect()` sets it true, `disconnect()` sets it false. `doConnect()` checks it before reconnecting.
  2. Added `wsId` counter: each WebSocket gets a unique ID. All callbacks (`onopen`, `onclose`, `onmessage`) check `myWsId !== this.wsId` to detect if they belong to a stale instance, and bail out if so.


## Build Configuration

### gradle.properties Key Settings
```properties
hermesEnabled=true
newArchEnabled=false
expo.useLegacyPackaging=false
android.enableProguardInReleaseBuilds=true
android.enableShrinkResourcesInReleaseBuilds=true
```

### Build Commands
```bash
# Prebuild (regenerates native Android project)
npx expo prebuild --platform android --clean

# Debug APK
cd android && ./gradlew assembleDebug

# Release APK
cd android && ./gradlew assembleRelease
```

## Build Issues Resolved
1. **Gradle 9.3 crash** → Downgraded to Gradle 8.14.3
2. **Kotlin version conflict** → Pinned 2.1.20
3. **Hermes path** → Fixed hermesc binary path in app/build.gradle
4. **Android SDK 36 missing** → Installed platform-36 + build-tools-36
5. **expo-av CMake failure** → Temporarily removed (pending fix)

## vs Desktop (Electron)

| Feature | Desktop | Mobile (RN) |
|---------|---------|-------------|
| Rendering | Electron + React | React Native |
| State | Zustand (same) | Zustand (same) |
| API | REST + WebSocket (same) | REST + WebSocket (same) |
| Backend | Go server (unchanged) | Go server (unchanged) |
| Voice | MediaRecorder API | expo-av Audio.Recording |
| Navigation | React state | React state (→ react-native-screens) |
| Storage | localStorage | expo-secure-store |
