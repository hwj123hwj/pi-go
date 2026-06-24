---
type: entity
date: 2026-06-14
tags: [config, environment-variables, settings]
---

# Configuration System

> Environment-driven configuration for pi-go agent server and its components.
> Load priority: 环境变量 > .env 文件 > Default() 硬编码值.

## Source

`internal/config/config.go` — Defines the `Config` struct loaded from environment variables via `LoadDotEnv` + `LoadFromEnv`.

## Config Struct

```go
type Config struct {
    // Provider & Model
    Provider     string  // PI_GO_PROVIDER (default: "", required at runtime — v7)
    Model        string  // PI_GO_MODEL

    // Anthropic
    AnthropicKey string  // ANTHROPIC_API_KEY
    AnthropicModel string // ANTHROPIC_MODEL
    AnthropicBaseURL string // ANTHROPIC_BASE_URL

    // OpenAI-compatible gateway
    OpenAIAPIKey  string  // PI_GO_API_KEY (preferred) / OPENAI_API_KEY (fallback)
    OpenAIModel   string  // PI_GO_MODEL (preferred) / OPENAI_MODEL (fallback)
    OpenAIBaseURL string  // PI_GO_BASE_URL (preferred) / OPENAI_BASE_URL (fallback)

    // Server
    Host string  // PI_GO_HOST (default: "127.0.0.1")
    Port string  // PI_GO_PORT (default: "8080")

    // Data
    DataDir    string // PI_GO_DATA_DIR (default: "./data")
    SessionFile string // PI_GO_SESSION_FILE

    // Tools
    EnableBash      bool   // PI_GO_ENABLE_BASH
    BashTimeoutSec  int    // PI_GO_BASH_TIMEOUT_SECONDS (default: 30)
    MaxOutputLen    int    // PI_GO_MAX_OUTPUT_LEN (default: 30000)

    // Workspace
    Workspace   string // PI_GO_WORKSPACE
    ExecutionMode string // PI_GO_EXECUTION_MODE (local/ssh)

    // SSH
    SSHHost    string // PI_GO_SSH_HOST
    SSHPort    string // PI_GO_SSH_PORT (default: "22")
    SSHWorkDir string // PI_GO_SSH_WORKDIR

    // Filtering
    AllowedTools string // PI_GO_ALLOWED_TOOLS (comma-separated)
    BlockedTools string // PI_GO_BLOCKED_TOOLS (comma-separated)

    // UI
    HistoryFile string // PI_GO_HISTORY_FILE

    // Prompts
    PromptTemplate string // PI_GO_PROMPT_TEMPLATE
}
```

## Key Configuration Groups

### Provider Selection
`PI_GO_PROVIDER` selects which LLM backend to use (**required**, no default):
- `anthropic` — Anthropic Messages API
- `openai` — OpenAI Chat Completions API (also compatible with local gateways)
- `mock` — ❌ Removed (v7); real provider is now required
- `deepv` — ❌ Removed (v6); local gateway handles all providers

> **v7 change**: Empty `PI_GO_PROVIDER` causes a startup error. There is no fallback to mock. `registerProviders()` in `app.go` returns an error for empty/unknown provider values.

### Gateway Mode
For local LLM gateways (e.g., `go-llm-gateway` on port 4001):
```
PI_GO_PROVIDER=openai
PI_GO_BASE_URL=http://localhost:4001
PI_GO_API_KEY=sk-local-gateway-key
PI_GO_MODEL=longcat-opus
```

### Environment Variable Priority (v6)
`LoadDotEnv` no longer overrides already-set environment variables:
1. **Shell environment variables** (highest priority)
2. **`.env` file values** (only if env var is empty)
3. **`Default()` hardcoded values** (fallback)

This prevents `.env` empty fields from zeroing out shell-set keys.

### Tool Sandboxing
- `PI_GO_ENABLE_BASH=false` — Disable shell execution
- `PI_GO_ALLOWED_TOOLS=read,write,grep` — Whitelist only
- `PI_GO_BLOCKED_TOOLS=bash,rm` — Blacklist specific tools

### Remote Execution
- `PI_GO_EXECUTION_MODE=ssh` — Switch to remote execution
- `PI_GO_SSH_HOST=server.example.com` — Remote host

## Integration

Config is loaded in `cmd/pi-agent/main.go` via `godotenv.Load()` + env var parsing, then passed to:
- `internal/app/app.go` — Dependency injection
- `internal/server/server.go` — HTTP server settings
- `internal/mode/` — Mode-specific options

## Related

- [[server-websocket]] — Server uses Config for Host/Port
- [[llm-provider-system]] — Provider selection driven by Config
- [[operations-abstract]] — ExecutionMode selects local vs SSH
