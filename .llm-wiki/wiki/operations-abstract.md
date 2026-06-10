---
type: concept
date: 2026-06-10
tags: [operations, local, ssh, execution-backend]
---

# Operations Abstraction

> Pluggable execution backends for file and bash operations.

## Interface

The `operations.Operations` interface provides:
- `BashOperations` — Run shell commands
- `FileOperations` — Read, write, edit files

## Implementations

| Backend | Status | Source |
|---------|--------|--------|
| `LocalOperations` | ✅ | Direct execution on local machine |
| `SSHOperations` | ✅ | Remote execution via SSH |

## Configuration

- `PI_GO_EXECUTION_MODE` — `local` (default) or `ssh`
- SSH: `PI_GO_SSH_HOST`, `PI_GO_SSH_PORT`, `PI_GO_SSH_WORKDIR`

## Related

- [[tool-system]] — Tools use Operations for execution
- [[runtime-application-interface]] — `ToolBuildOptions` passes BashOps and FileOps