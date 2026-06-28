---
type: source
source_path: "."
date: 2026-06-28
tags: [asr, voice-input, speech-to-text, siliconflow, telespeech, media-recorder, mobile, capacitor, promptbar]
---

# Source: Project Root — v40 (Voice Input / ASR)

> Voice input feature: microphone recording in browser → server-side proxy to SiliconFlow TeleSpeechASR → text echoed to input box.

## Key Takeaways

1. **Full-stack ASR pipeline** — Audio is captured client-side via `MediaRecorder` API, uploaded as multipart form data to the Go server, which proxies it to SiliconFlow's `TeleAI/TeleSpeechASR` model. The API key never touches the frontend.

2. **Server proxy pattern** — The `ASRHandler` in `internal/server/asr.go` receives the audio file, rebuilds a multipart request with the `model` field and `Authorization` header, forwards it to `https://api.siliconflow.cn/v1/audio/transcriptions`, and parses the response. This is the same pattern used for KB embeddings (KB vector search also proxies through SiliconFlow).

3. **Config reuse** — ASR defaults to reusing `SILICONFLOW_API_KEY` if `ASR_API_KEY` is not explicitly set. The model defaults to `TeleAI/TeleSpeechASR`. Both the base URL and model are overridable via env vars.

4. **Frontend hook architecture** — `useVoiceInput.ts` is a reusable React hook that encapsulates: `MediaRecorder` lifecycle (start/stop, codec detection), blob assembly, FormData upload, and transcription result callback. The `PromptBar` component uses it to append transcribed text to the textarea.

5. **Security: API key isolation** — The SiliconFlow API key is stored only on the server (env var). The frontend never sees it — all ASR requests go through the Go backend's `/asr/transcribe` endpoint.

6. **Android permission** — `RECORD_AUDIO` permission added to `AndroidManifest.xml` for microphone access on mobile.

## Important Entities

- [[desktop-app]] — Voice input button in PromptBar, useVoiceInput hook
- [[server-websocket]] — New `/asr/transcribe` REST endpoint
- [[config-system]] — New ASR config fields (ASRAPIKey, ASRModel, ASRBaseURL)
- [[asr-voice-input]] — New concept page for the ASR pipeline

## Technical Details

### Backend (`internal/server/asr.go`)

```
Client → POST /asr/transcribe (multipart: file=audio.webm)
  → ASRHandler.transcribe()
    → ParseMultipartForm (25MB max)
    → Rebuild multipart with model field
    → POST https://api.siliconflow.cn/v1/audio/transcriptions
    → Parse response: { "text": "..." } or { "data": { "text": "..." } }
  → { "text": "识别结果" }
```

### Frontend (`desktop/src/hooks/useVoiceInput.ts`)

```
User clicks 🎤 → navigator.mediaDevices.getUserMedia({ audio: true })
  → MediaRecorder (prefers audio/webm;codecs=opus)
  → User clicks again → mr.stop()
    → Blob assembled from chunks
    → FormData upload to /asr/transcribe
    → onText(result) → appendText to textarea
```

### Config fields

| Field | Env Var | Default |
|-------|---------|---------|
| `ASRAPIKey` | `ASR_API_KEY` (falls back to `SILICONFLOW_API_KEY`) | — |
| `ASRModel` | `ASR_MODEL` | `TeleAI/TeleSpeechASR` |
| `ASRBaseURL` | `ASR_BASE_URL` | `https://api.siliconflow.cn` |

### Files Changed

| File | Change |
|------|--------|
| `internal/server/asr.go` | **NEW** — ASR proxy handler (163 lines) |
| `internal/config/config.go` | Added 3 ASR config fields + env loading |
| `internal/server/server.go` | Registered `/asr/` route prefix + ASR handler |
| `desktop/src/hooks/useVoiceInput.ts` | **NEW** — React hook for mic recording + ASR |
| `desktop/src/components/PromptBar.tsx` | Added 🎤 voice button, transcribing state, error toast |
| `desktop/src/components/Icon.tsx` | Added `mic` icon (24×24 SVG path) |
| `desktop/src/styles/app.css` | `.btn-voice` styles (desktop + mobile), recording pulse animation |
| `desktop/android/app/src/main/AndroidManifest.xml` | Added `RECORD_AUDIO` permission |
| `desktop/android/app/build.gradle` | Version bump: `0.8.0` (versionCode 8) |

## Cross-References

- [[desktop-app]] — PromptBar component, mobile platform features
- [[config-system]] — ASR config fields
- [[server-websocket]] — REST endpoint for ASR
- [[kb-vector-search]] — Same SiliconFlow API key reuse pattern
