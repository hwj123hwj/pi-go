---
type: entity
date: 2026-07-13
tags: [deployment, ci-cd, systemd, github-actions, infrastructure, release, cross-compile]
related: [[config-system]], [[feishu-integration]], [[server-websocket]], [[tui-bubbletea]]
---

# Deployment Infrastructure

> pi-go uses **two GitHub Actions workflows**: one for continuous server deployment, one for cross-platform release binary distribution.

## Release Workflow (v0.10.0+) — `.github/workflows/release.yml`

**Trigger**: Push of version tags (`v*`) or manual `workflow_dispatch`.

**Pipeline steps**:
1. Checkout + setup Go 1.24.2
2. Run tests (`go test ./...`)
3. Cross-compile for 4 platforms:
   - `linux/amd64` — pi-agent + pi-feishu-bridge
   - `linux/arm64` — pi-agent
   - `darwin/amd64` — pi-agent
   - `darwin/arm64` — pi-agent
4. Version injection via `-ldflags "-X main.version=<tag>"`
5. Generate SHA256 checksums
6. Create GitHub Release (via `softprops/action-gh-release@v2`) with binaries attached

**Used by**: `scripts/install.sh` downloads these pre-compiled binaries for one-line installation. See [[tui-bubbletea]] for installer details.

## Deploy Workflow — `.github/workflows/deploy.yml`

**Trigger**: Push to `main` branch or manual `workflow_dispatch`.

**Pipeline steps**:

1. `go test ./...` — Full test suite
2. Build `linux/amd64` binaries (CGO_ENABLED=0):
   - `pi-agent` — Main agent binary
   - `pi-feishu-bridge` — Feishu bridge binary
3. Package into tarball (`release-<git-sha>.tar.gz`)
4. SCP to server `/tmp/`
5. Render + install systemd service files (`pi-go.service`, `pi-feishu-bridge.service`)
6. Update `current` symlink to new release
7. Restart services via `systemctl`
8. Health check: `curl http://127.0.0.1:8080/health`

## Server Layout

```
/opt/pi-go/
├── current -> /opt/pi-go/releases/release-<git-sha>
├── releases/
│   └── release-<git-sha>/
│       ├── pi-agent
│       ├── pi-feishu-bridge
│       └── README.md
└── shared/
    ├── .env
    └── data/
        └── session.jsonl
```

## Systemd Services

Two services managed independently:
- **pi-go** — Main agent service (serves HTTP on `127.0.0.1:8080`)
- **pi-feishu-bridge** — Feishu bridge service (connects to pi-go HTTP API)

Service files use `__DEPLOY_PATH__` placeholder, replaced at deploy time via `sed`.

## GitHub Secrets Required

| Secret | Purpose |
|--------|---------|
| `DEPLOY_HOST` | Server IP (e.g. `8.141.97.21`) |
| `DEPLOY_USER` | SSH user (e.g. `root`) |
| `DEPLOY_PORT` | SSH port (default `22`) |
| `DEPLOY_PATH` | Install path (e.g. `/opt/pi-go`) |
| `DEPLOY_SSH_KEY` | SSH private key content |
| `PI_GO_ENV` | Server `.env` file content |

## Architecture Notes

- Agent service only listens on `127.0.0.1:8080` (no public exposure)
- Feishu bridge runs on same machine, communicates via localhost HTTP
- If switching to long-connection event mode for Feishu, same deployment works (no extra ports needed)
- Binary is cross-compiled (`GOOS=linux GOARCH=amd64 CGO_ENABLED=0`), no server-side compilation needed

## Source

- `.github/workflows/release.yml` — Cross-platform release binary workflow
- `.github/workflows/deploy.yml` — Server deployment workflow
- [docs/deploy.md](../../docs/deploy.md) — Deployment documentation
- `deploy/` directory — Systemd service file templates
- `scripts/install.sh` — One-line installer (downloads release binaries)
- `scripts/install_release.sh` — Server-side installation script
