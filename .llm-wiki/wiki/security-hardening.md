# Security Hardening: SSRF, API Auth, YAML Config, Version

> Source: `internal/util/ssrf.go`, `internal/server/server.go`, `internal/config/config.go`
> Date: 2026-07-05

## Overview

Five security and operational improvements:

1. **SSRF Protection on External Tool Callbacks** — blocks cloud metadata, private IPs
2. **HTTP API Bearer Token Authentication** — configurable auth middleware
3. **YAML Config File Support** — `pi-go.yaml` alternative to 30+ env vars
4. **`--version` Flag & `/health` Version** — build traceability
5. **Feishu OAuth Scan Login** — `/feishu setup` QR code login flow

---

## 1. SSRF Protection on External Tools

### Problem

`validateCallbackURL()` only checked the URL scheme. An attacker could register an external tool with `callback_url: "http://169.254.169.254/latest/meta-data/"` and the agent would POST to it, exfiltrating cloud credentials.

### Solution

Shared SSRF utilities extracted to `internal/util/ssrf.go`:
- `IsPrivateHost()` — DNS-resolves hostname, checks all IPs
- `IsPrivateIP()` — blocks loopback, private, link-local, cloud metadata
- `ValidateCallbackURL()` — full SSRF check (scheme + credentials + private host)

`ExternalToolDef` now has `AllowPrivate bool` field:
- `false` (default): full SSRF protection — blocks all private addresses
- `true`: trusted internal services (e.g. Feishu bridge on localhost)

### Backward Compatibility

Existing Feishu bridge registrations that use `localhost` must set `AllowPrivate: true`.
New registrations to public URLs are unaffected.

---

## 2. HTTP API Authentication

### Architecture

```
Request → CORS → authMiddleware → recovery → logging → handler
                        │
                        ▼
              ┌── apiKey empty? → pass through (backward compatible)
              ├── /health?      → pass through (monitoring)
              ├── Bearer match? → pass through
              └── else          → 401 Unauthorized
```

### Configuration

```bash
# Enable API auth (empty = no auth, backward compatible)
export PI_GO_API_KEY=my-secret-token

# Or in pi-go.yaml:
# api_key: my-secret-token
```

Clients must send: `Authorization: Bearer my-secret-token`

### Security Impact

| Endpoint | Before | After (with API key) |
|----------|--------|---------------------|
| `POST /chat` | Open RCE | Auth required |
| `PUT /workspace/write-file` | Arbitrary write | Auth required |
| `GET /workspace/read-file` | Arbitrary read | Auth required |
| `GET /health` | Open | Open (monitoring) |

---

## 3. YAML Config File Support

### Precedence (highest to lowest)

1. Environment variables (`PI_GO_*`)
2. `.env` file
3. YAML config file (`pi-go.yaml` or `~/.pi-go/config.yaml`)
4. Defaults

### Example `pi-go.yaml`

```yaml
# Provider
provider: anthropic
anthropic_api_key: sk-ant-...
anthropic_model: claude-sonnet-4-6

# Tools
workspace: /home/user/my-project
enable_bash: true
enable_web: true
enable_web_search: true

# SSH remote execution
execution_mode: ssh
ssh_host: user@server
ssh_port: 22
ssh_work_dir: /home/user/project

# Security
api_key: my-secret-token

# Knowledge base
kb_repo_path: ~/agent-lessons
kb_embedding_api_key: sk-sf-...
kb_embedding_model: BAAI/bge-m3

# ASR
asr_model: TeleAI/TeleSpeechASR
```

### Auto-Detection

`--config` flag → `pi-go.yaml` in CWD → `~/.pi-go/config.yaml`

---

## 4. Version Flag & Health Endpoint

### `--version` Flag

```bash
$ pi-agent --version
pi-go v1.0.0-5-gabc1234
```

Version is injected at build time:
```bash
go build -ldflags "-X main.version=$(git describe --tags)" ./cmd/pi-agent/
```

### `/health` Response

```json
{
  "status": "ok",
  "version": "v1.0.0-5-gabc1234"
}
```

---

## Test Results

| Package | Status |
|---------|--------|
| `internal/agent/` | ✅ 9 tests (updated for new API) |
| `internal/tools/` | ✅ All pass |
| `internal/config/` | ✅ All pass |
| `internal/server/` | ✅ All pass |
| Full suite (33 packages) | ✅ 0 failures |
