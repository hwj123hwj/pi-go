---
type: entity
date: 2026-06-10
tags: [pi-go, entity, feishu, bridge, integration]
source: "source-project-root.md"
---

# Feishu Bridge (`internal/feishu/`)

Standalone service (cmd/pi-feishu-bridge) connecting pi-agent to Feishu/Lark group chat. Communicates via HTTP API.

## Three Core Components

### Client
- Wraps larksuite OAPI SDK v3 for REST operations
- Reply/send/update message, add/remove emoji reactions
- Send Markdown (post type), images, files
- Upload via multipart, token cache with TTL - 60s refresh

### Gateway
- WebSocket long connection to Feishu event subscription
- Handles text, image (download + save locally), post (rich text to markdown)
- Dedup: messageID map (max 1000), content + time window (5s)
- Text Choice Waiter: numbered options, blocks until user reply

### Handler
- Routes messages: local slash commands or forward to pi-agent
- Two streaming modes: CardKit (preferred) vs text PATCH fallback
- SSE handling: text_delta, tool_start, done, error
- Auto-uploads detected files from LLM replies
- /project create: creates group + binds project directory
- Routes persisted to ~/.pi-go/feishu-routes.json

## Slash Commands

/new, /compact, /compress, /status, /help, /project

## [[wikilinks]]

- App Layer
- Agent Loop
