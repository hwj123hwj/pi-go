---
type: entity
date: 2026-06-10
tags: [pi-go, entity, app-layer, assembly]
source: "source-project-root.md"
---

# App Layer (`internal/app/`)

The thin dependency assembly layer. Wires providers, session manager, extension registry, and the Application implementation.

## Role in Architecture

Sits in the Entrypoints layer (between CLI/server code and the Application/Platform layers).

## Key Responsibilities

- **Provider registration**: Reads config, registers selected LLM provider (fails loudly on missing keys)
- **Session lifecycle**: NewSession, LoadSession, LoadOrCreateSession via SessionRegistry cache
- **External tool registration**: SetExternalTools for bridge-registered HTTP tools
- **Operations backend**: Selects local vs SSH Operations based on ExecutionMode config
- **Slash command context**: Implements slashcmd.AppContext
- **Tool filtering**: Applies AllowedTools/BlockedTools from config

## Key Design

- Application implementation injected via AppOptions.Application (dependency inversion)
- deps() constructs runtime.Dependencies on each session creation
- Application defaults to CodingApplication if not injected

## [[wikilinks]]

- Layer Architecture
- AI Providers
- Extension System
