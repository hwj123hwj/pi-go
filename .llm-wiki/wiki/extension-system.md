---
type: entity
date: 2026-06-10
tags: [extensions, plugins, hooks]
---

# Extension System

> Plugin-style extension framework for adding tools, commands, and event hooks.

## Extension Interface

Extensions can provide:
- **Tools** — Custom tools registered via `Tools()` method
- **Commands** — Custom slash commands
- **Event Hooks** — React to agent lifecycle events

## Integration

- Extensions are loaded at application startup
- Extension tools are passed through `ToolBuildOptions.ExtensionTools` to [[runtime-application-interface]]
- Duck typing allows extensions to implement `ConcurrencySafeChecker` without registration

## Related

- [[tool-system]] — Extensions can add tools
- [[slash-command-framework]] — Extensions can add commands
- [[runtime-application-interface]] — Extension tools flow through ToolBuildOptions