---
type: concept
date: 2026-06-28
tags: [asr, voice-input, speech-to-text, siliconflow, telespeech, media-recorder, audio]
related: [[desktop-app]], [[server-websocket]], [[config-system]], [[kb-vector-search]]
---

# ASR Voice Input

> Speech-to-text pipeline: browser microphone → server proxy → SiliconFlow TeleSpeechASR → text in input box.
> Added in v40 (2026-06-28).

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Frontend (React)                                             │
│                                                              │
│  PromptBar.tsx                                               │
│    └── 🎤 button → useVoiceInput() hook                     │
│         ├── MediaRecorder API (audio/webm;codecs=opus)      │
│         ├── Blob assembly from chunks                       │
│         └── FormData upload → POST /asr/transcribe          │
│                                                              │
│  On success: onText(transcription) → appendText(textarea)   │
│  On error: voiceError toast (permission denied, ASR fail)   │
└──────────────────────────┬──────────────────────────────────┘
                           │ multipart/form-data
                           │ (file=voice.webm)
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ Backend (Go)                                                 │
│                                                              │
│  ASRHandler (internal/server/asr.go)                        │
│    ├── ParseMultipartForm (25MB max)                        │
│    ├── Rebuild multipart: model + file                      │
│    ├── POST https://api.siliconflow.cn/v1/audio/            │
│    │       transcriptions                                    │
│    │   Authorization: Bearer <ASR_API_KEY>                   │
│    └── Parse: { "text": "..." } or { "data": {...} }        │
│                                                              │
│  Response: { "text": "识别结果" }                             │
└─────────────────────────────────────────────────────────────┘
```

## Key Design Decisions

### 1. Server-Side Proxy (Not Direct Frontend→SiliconFlow)

The frontend never calls SiliconFlow directly. Instead, audio goes through the Go server's `/asr/transcribe` endpoint. This:
- **Protects the API key** — `SILICONFLOW_API_KEY` stays server-side only
- **Follows existing patterns** — KB embeddings use the same proxy approach ([[kb-vector-search]])
- **Enables future server-side processing** — Caching, rate limiting, audio preprocessing

### 2. API Key Reuse

ASR defaults to reusing `SILICONFLOW_API_KEY` (already configured for KB embeddings). This means **zero additional configuration** for existing deployments — ASR works out of the box.

| Config | Env Var | Default |
|--------|---------|---------|
| `ASRAPIKey` | `ASR_API_KEY` | Falls back to `SILICONFLOW_API_KEY` |
| `ASRModel` | `ASR_MODEL` | `TeleAI/TeleSpeechASR` |
| `ASRBaseURL` | `ASR_BASE_URL` | `https://api.siliconflow.cn` |

### 3. MediaRecorder Codec Selection

The hook detects the best supported codec in order:
1. `audio/webm;codecs=opus` (Chrome, Android — best compression)
2. `audio/webm` (Chrome fallback)
3. Browser default (Safari — likely `audio/mp4`)

### 4. Click-to-Toggle (Not Push-to-Talk)

The mic button uses a **toggle** interaction:
- Click → start recording (button turns red, pulsing animation)
- Click again → stop → auto-transcribe → text appended

This was chosen over push-to-talk because:
- No need to hold the button on mobile
- Clear visual feedback (red pulse = recording)
- Familiar pattern (like WeChat voice messages, but transcribed)

## Files

| File | Role |
|------|------|
| `internal/server/asr.go` | Backend proxy handler |
| `internal/config/config.go` | ASR config fields + env loading |
| `internal/server/server.go` | Route registration (`/asr/`) |
| `desktop/src/hooks/useVoiceInput.ts` | React hook (MediaRecorder + upload) |
| `desktop/src/components/PromptBar.tsx` | 🎤 button UI + transcribing state |
| `desktop/src/components/Icon.tsx` | `mic` icon SVG |
| `desktop/src/styles/app.css` | `.btn-voice` styles + pulse animation |
| `desktop/android/app/src/main/AndroidManifest.xml` | `RECORD_AUDIO` permission |

## Related

- [[desktop-app]] — PromptBar, mobile platform support
- [[server-websocket]] — REST API endpoint `/asr/transcribe`
- [[config-system]] — ASR config fields
- [[kb-vector-search]] — Same SiliconFlow API key reuse pattern
