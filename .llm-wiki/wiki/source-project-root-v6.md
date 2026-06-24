---
type: source
source_path: "."
date: 2026-06-23
tags: [config, deepv-removed, gateway, env-vars, longcat-opus, load-dotenv, pi-go-api-key, loadenv-priority]
related: [[config-system]], [[llm-provider-system]], [[source-project-root-v5]], [[overview]]
---

# Source: Project Root v6

> Re-ingest after config system overhaul, DeepV removal, and default model change.
> Covers commits `148b0e4..669cff5` (2026-06-23).

## Key Takeaways

1. **DeepV provider removed entirely** — `internal/ai/providers/deepv.go` (469 lines), `internal/agents/coding/deepv/headers.go` (128 lines), and `headers_test.go` (29 lines) deleted. All references removed from `config.go`, `app.go`, `agent_session.go`, `server.go`, `application.go`. Net -688 lines.
2. **Default model changed to `longcat-opus`** — Free, fast model for testing. Was `mimo-opus` (DeepV-specific). `config.Default()` now sets `OpenAIModel: "longcat-opus"`.
3. **New env var naming convention** — `PI_GO_API_KEY` is now the preferred name for the gateway API key. `OPENAI_API_KEY` still accepted as fallback. Same pattern for `PI_GO_MODEL` / `OPENAI_MODEL` and `PI_GO_BASE_URL` / `OPENAI_BASE_URL`.
4. **`LoadDotEnv` no longer overrides existing env vars** — Priority chain: `环境变量 > .env 文件 > Default()`. Previously `.env` would `os.Setenv` and overwrite already-set environment variables, causing 401 errors when shell had a different key.
5. **Gateway process identified** — Local gateway is `go-llm-ga` (PID 71067), listening on `localhost:4001` (service name `newoak`). Handles Anthropic/OpenAI/Responses protocol translation.
6. **`.env.example` updated** — Uses `PI_GO_API_KEY=sk-xxxx` instead of `OPENAI_API_KEY=sk-local-gateway-hwj123hwj`. Generic placeholder prevents accidental key leakage.
7. **`registerProviders` error message updated** — Now says `PI_GO_PROVIDER=openai but OPENAI_API_KEY is empty` (was generic).

## Important Entities & Concepts

### Config System Overhaul
- `config.Default()`: `OpenAIAPIKey = "sk-local-gateway-hwj123hwj"`, `OpenAIBaseURL = "http://localhost:4001"`, `OpenAIModel = "longcat-opus"`
- `LoadFromEnv()`: Dual-read pattern — checks `PI_GO_*` first, falls back to `OPENAI_*`
- `LoadDotEnv()`: Now skips keys where `os.Getenv(key) != ""` — preserves shell-set env vars
- `.env.example`: Uses `PI_GO_PROVIDER=openai`, `PI_GO_API_KEY=sk-xxxx`, `PI_GO_MODEL=longcat-opus`, `PI_GO_BASE_URL=http://localhost:4001`

### Provider Registry State
- **Removed**: `deepv` provider (was 469 + 128 lines)
- **Remaining**: `mock`, `anthropic`, `openai`
- `registerProviders()` in `app.go`: switch on `cfg.Provider` — `anthropic`, `openai`, `mock`, default (error)
- Mock provider always registered as fallback

### Gateway Architecture
- `go-llm-gateway` process on `localhost:4001`
- Three POST endpoints: `/v1/messages` (Anthropic), `/v1/chat/completions` (OpenAI), `/v1/responses` (OpenAI Responses)
- `/v1/models` returns real model list, `/health` for health check
- All models accessed via OpenAI-compatible format through gateway — no special providers needed

## Notable Code References

| File | Change |
|------|--------|
| `go.mod` | Module path: `github.com/hwj123hwj/pi-go` |
| `internal/config/config.go` | Default model → longcat-opus, PI_GO_* env priority, LoadDotEnv no-override |
| `internal/ai/providers/deepv.go` | DELETED (469 lines) |
| `internal/agents/coding/deepv/headers.go` | DELETED (128 lines) |
| `internal/agents/coding/deepv/headers_test.go` | DELETED (29 lines) |
| `internal/app/app.go` | DeepV references removed from registerProviders |
| `.env.example` | PI_GO_* naming, placeholder key |
| `cmd/pi-agent/main.go` | LoadDotEnv → LoadFromEnv sequence unchanged |

## Contradictions with Existing Wiki

- **[[llm-provider-system]]**: Lists DeepV as a supported provider. **Now removed** — only `mock`, `anthropic`, `openai` remain.
- **[[config-system]]**: Documents `DEEPV_ENABLED`, `DEEPV_SERVER_URL`, `DEEPV_MODEL`, `DEEPV_WORK_DIR` fields and DeepV provider selection. **All removed**. Also documents `godotenv.Load()` which is now `config.LoadDotEnv()`. Config struct no longer has DeepV fields.
- **[[config-system]]**: Documents `PI_GO_MODEL` as primary env var. Now `PI_GO_API_KEY` is the new preferred naming for the API key (not just model).
- **[[overview.md]]**: Lists "多 Provider: Anthropic / OpenAI / DeepV / Mock". **DeepV removed** — now "Anthropic / OpenAI / Mock".
- **[[overview.md]]**: Lists `OPENAI_API_KEY` in gateway mode example. Now `PI_GO_API_KEY` is preferred.
