# Feishu OAuth Scan Login

> Source: `internal/feishu/oauth.go`, `internal/feishu/credentials.go`, `internal/agents/coding/commands/feishu.go`
> Date: 2026-07-05

## Overview

Implements Feishu (飞书) OAuth scan login flow, modeled after hwjcode's `/feishu setup` command.
Users scan a QR code with the Feishu mobile app to authorize pi-go, eliminating the need
to manually configure `FEISHU_APP_ID` / `FEISHU_APP_SECRET` environment variables.

## Architecture

```
User: /feishu setup
         │
         ▼
StartOAuthFlow()
         │
         ├── Start local HTTP server (random port)
         ├── Open browser → open.feishu.cn/open-apis/authen/v1/authorize
         │                  ?app_id=cli_a94f42eb71f9dccc
         │                  &redirect_uri=http://localhost:PORT/callback
         │
         │    ┌─── User scans QR with Feishu App ───┐
         │    │                                      │
         │    ▼                                      │
         │  Feishu OAuth page                        │
         │    │                                      │
         │    ▼                                      │
         │  Redirect → localhost:PORT/callback       │
         │            ?code=AUTH_CODE                │
         │                                      │
         ▼ ◄────────────────────────────────────────┘
  handleOAuthCallback()
         │
         ├── getAppAccessToken(app_id, app_secret)
         │     POST /open-apis/auth/v3/app_access_token/internal
         │
         ├── getUserAccessToken(code, app_access_token)
         │     POST /open-apis/authen/v1/oidc/access_token
         │
         ├── getUserInfo(user_access_token)
         │     GET /open-apis/authen/v1/user_info
         │
         ▼
  Save credentials → ~/.pi-go/feishu-credentials.json
         │
         ▼
  User: /feishu start → pi-feishu-bridge auto-loads credentials
```

## Pre-Registered App

pi-go uses a pre-registered Feishu app (built into the binary):
- App ID: `cli_a94f42eb71f9dccc`
- App Secret: stored as constant in `oauth.go`

This is the same pattern hwjcode uses (`cli_aa9c19096a7c9cc5`). Users don't need
to create their own app — they just scan and authorize.

## Slash Commands

| Command | Description |
|---------|-------------|
| `/feishu` | Show help |
| `/feishu setup` | QR scan OAuth login (opens browser) |
| `/feishu setup --manual <id> <secret>` | Manual credentials |
| `/feishu start` | Show instructions to start the bot |
| `/feishu stop` | Stop the running bot |
| `/feishu status` | Show current status and credentials |
| `/feishu logout` | Clear credentials and disconnect |

## Credential File

```
~/.pi-go/feishu-credentials.json
```

```json
{
  "app_id": "cli_a94f42eb71f9dccc",
  "app_secret": "GJkOZ...",
  "user_open_id": "ou_xxxx",
  "user_access_token": "u-xxx",
  "bot_name": "黄威健",
  "platform": "feishu"
}
```

File permissions: `0600` (owner-only read/write).

## pi-feishu-bridge Integration

The bridge now auto-loads credentials from the file:
1. Check `FEISHU_APP_ID` / `FEISHU_APP_SECRET` env vars (legacy)
2. If empty, fall back to `~/.pi-go/feishu-credentials.json`
3. If neither, exit with helpful error message

## Feishu API Endpoints Used

| Endpoint | Purpose |
|----------|---------|
| `GET /open-apis/authen/v1/authorize` | OAuth authorize (user scans QR) |
| `POST /open-apis/auth/v3/app_access_token/internal` | Get app token |
| `POST /open-apis/authen/v1/oidc/access_token` | Exchange code for user token |
| `GET /open-apis/authen/v1/user_info` | Get authenticated user info |

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Pre-registered app_id | Same as hwjcode — users don't need to create apps |
| Local callback server | Standard OAuth redirect flow (localhost:random_port) |
| Credential file (JSON) | Survives restarts; user doesn't re-scan every time |
| Browser auto-open | Best UX — xdg-open / open / rundll32 |
| 5-minute timeout | Prevents zombie OAuth servers |

## Test Results

All 33 packages pass tests.
