---
type: entity
date: 2026-06-20
tags: [tools, web, fetch, ssrf, http]
---

# Web Fetch Tool

> The 8th built-in tool in pi-go. Fetches a URL's content and converts it to markdown. Inspired by Claude Code's WebFetchTool.

## Overview

`web_fetch` allows the agent to read public web pages — documentation, API references, articles — by fetching the HTML and converting it to markdown. It includes comprehensive SSRF protection to prevent the agent from being tricked into accessing internal resources.

## Tool Signature

| Field | Value |
|-------|-------|
| Name | `web_fetch` |
| Parameters | `url` (required, string), `prompt` (optional, string) |
| Location | `internal/tools/web_fetch.go` |

## Security: SSRF Protection

The security model is defined in `internal/tools/web_fetch_security.go`:

### URL Validation (`validateURL`)
1. Length ≤ 2048 characters
2. Protocol must be `http` or `https`
3. No username/password in URL (prevents credential leakage)
4. Hostname must have ≥ 2 segments (or be `localhost`)

### Host Filtering (`isPrivateHost`)
Blocks access to:
- `localhost` / `*.localhost` / `*.local`
- Loopback, private, link-local, unspecified IPs
- Cloud metadata addresses (169.254.169.254)
- DNS-resolved domains that point to private IPs

### Redirect Protection
Every HTTP redirect is checked via `client.CheckRedirect` — if a 302 redirects to a private host, the request is rejected. This prevents "302 bypass" attacks.

## Execution Flow

```
URL input → validateURL() → isPrivateHost(entry) → HTTP GET
  → CheckRedirect(per hop) → Content-Type check (text/html only)
  → io.LimitReader(10MB) → html-to-markdown → TruncateOutput
```

## Dependencies

| Dependency | Purpose |
|------------|---------|
| `html-to-markdown` (`github.com/JohannesKaufmann/html-to-markdown`) | HTML → Markdown conversion |
| `net/http` | HTTP client with redirect checking |
| `net/url` + `net` | URL parsing and IP classification |

## Configuration

```go
type WebFetchToolOption func(*WebFetchTool)

WithWebFetchTimeout(seconds int)      // Default: 30s
WithWebFetchMaxOutputLen(n int)       // Default: DefaultMaxOutputLen
```

## Limitations (v1)

- Only `text/html` content type supported (images, JSON, PDF return error)
- `prompt` parameter is recorded but not used for content extraction
- No JavaScript rendering (static HTML only)
- No authentication support (public pages only)

## Related

- [[tool-system]] — 8th entry in the built-in tools table
- [[agent-core]] — Registered as a standard tool
- [[coding-application]] — Available in coding-agent toolset
