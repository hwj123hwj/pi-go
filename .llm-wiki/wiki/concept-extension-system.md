---
type: concept
date: 2026-06-10
tags: [pi-go, concept, extension, plugin, hook]
source: "source-project-root.md"
---

# Extension System (`internal/extensions/`)

In-process extension system for third-party code. MVP: compile-time registration only.

## Extension Interface

Extension can contribute: Tools, Commands, Hooks

## Optional: Lifecycle Hooks

ExtensionWithLifecycle adds BeforeToolCallHooks and AfterToolCallHooks.
These feed into agent.ToolLifecycleHooks.

## Tool Lifecycle (per execution)

Validate -> PrepareArguments (opt) -> Before hooks -> Execute -> After hooks -> Result

- BeforeToolCallHook: receives ToolCallContext, returns modified or error
- AfterToolCallHook: receives ToolResult, returns modified or error

## Registry

Register(ext) stores extension and auto-collects lifecycle hooks.
Tools(), Commands() aggregate across all extensions.

## Known Limitations

- No dynamic loading (.so), compile-time only
- No per-session extension scoping
- Hook ordering: registration order, no priority

## [[wikilinks]]

- App Layer
- Tool System
- Agent Loop
