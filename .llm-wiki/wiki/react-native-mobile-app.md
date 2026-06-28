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
- **react-native-screens**: Currently using manual state-based screen switching. Should migrate to `@react-navigation/native-stack` for native screen stack
- **TextInput uncontrolled**: ChatScreen input is controlled. For voice append, consider ref-based approach (js-uncontrolled-components)
- **useDeferredValue**: For expensive transcript rendering during streaming (js-concurrent-react)

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
