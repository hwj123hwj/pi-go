---
type: entity
date: 2026-06-10
tags: [tools, interface, execution]
---

# Tool System

> The tool abstraction that allows LLMs to interact with the environment.

## Core Interface

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any
    Validate(params json.RawMessage) (json.RawMessage, error)
    Execute(ctx context.Context, params json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error)
}
```

## Optional Interfaces

| Interface | Purpose |
|-----------|---------|
| `ToolWithMode` | Override default execution mode (parallel/sequential) |
| `ToolWithPromptInfo` | Provide system prompt snippets and guidelines |
| `ConcurrencySafeChecker` | Declare concurrency safety per invocation |

## Built-in Tools (7)

All located in `internal/tools/`:

| Tool | File | Description |
|------|------|-------------|
| `bash` | `bash.go` | Shell command execution |
| `read` | `read.go` | File reading |
| `write` | `write.go` | File writing |
| `edit` | `edit.go` | String-based file editing (batch support) |
| `grep` | `grep.go` | Content search |
| `find` | `find.go` | File search |
| `ls` | `ls.go` | Directory listing |

## Tool Lifecycle Hooks

The lifecycle system provides `Before/After` hooks and `PrepareArguments`:
- Source: `[[tool-lifecycle-hooks]]`
- Execution flow: Validate → PrepareArguments → Before hooks → Execute → After hooks

## Execution Model

The `partitionToolCalls` function groups tool calls into parallel-safe batches:
- Concurrency-safe tools → parallel batch
- Sequential or unsafe tools → serial batch
- Extension tools can implement `ConcurrencySafeChecker` via duck typing

### Partitioning Algorithm (`partition_test.go`)
The partition system (`internal/agent/partition_test.go`) classifies each tool call:
- **Safe** — Can run concurrently with other safe calls
- **Unsafe** — Must be serialized (file mutations, bash commands)
- Execution flow: partition → batch → validate → execute → collect

## Filtering

Tools can be filtered via config:
- `AllowedTools` — whitelist
- `BlockedTools` — blacklist

## External Tools

Tools can also be registered externally via HTTP callbacks ([[external-tools]]):
- Registered via `POST /tools/register` API endpoint
- Executed by HTTP POST to the callback URL
- Support streaming partial results

## Related

- [[agent-core]] — Tools are registered in the Agent
- [[runtime-application-interface]] — `BuildTools()` assembles the tool list
- [[llm-provider-system]] — Tool definitions are sent to LLMs