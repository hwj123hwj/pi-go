---
type: source
source_path: "."
date: 2026-06-24
tags: [mock-removed, provider-cleanup, config-overhaul, test-infrastructure, ci-fix]
related: [[config-system]], [[llm-provider-system]], [[coding-application]], [[source-project-root-v6]], [[overview]]
---

# Source: Project Root v7

> Re-ingest after Mock provider removal, test infrastructure cleanup, and CI fix.
> Covers commits `684357d..HEAD` (2026-06-24).

## Key Takeaways

1. **Mock provider completely removed** — `internal/ai/providers/mock.go` (127 lines) deleted. All references removed from `app.go`, `config.go`, `agent_session.go`, `application.go`, test files, `.env.example`, `install_release.sh`. The string `"mock"` no longer appears in any `.go` file.
2. **`registerProviders` no longer has fallback behavior** — Empty `PI_GO_PROVIDER` now returns an error (`"PI_GO_PROVIDER is not set (valid values: anthropic, openai)"`) instead of silently defaulting to mock. Only `anthropic` and `openai` are valid provider values.
3. **Runtime mock fallback removed** — `ModelInfo()` and `buildAgent()` in `agent_session.go` no longer force provider/model to `"mock"` when empty. Empty model now returns empty strings (error state).
4. **Test infrastructure updated** — `server_test.go` `newTestApp` now sets `Provider: "openai"` + `OpenAIAPIKey: "test-key"` to satisfy the new non-empty provider requirement. `TestServer_Chat` skips when gateway is unavailable.
5. **Config test unchanged** — `config_test.go` `TestDefault` asserts `Provider == ""` which is still valid (the default is empty, but `registerProviders` will error if actually used with empty provider).
6. **AvailableModels cleaned** — `CodingApplication.AvailableModels()` lists only real providers: `anthropic` (3 models) and `openai` (2 models). No mock entry.
7. **Deployment scripts updated** — `install_release.sh` uses `PI_GO_PROVIDER=openai` (was `mock`). `.env.example` comment says `anthropic / openai` (no mock).
8. **All tests pass** — `go test ./...` exits 0. `go vet ./images` exits 0.

## Important Entities & Concepts

### Provider System Cleanup
- `registerProviders()` in `app.go`: Only `anthropic` and `openai` cases remain
- Error on empty: `"PI_GO_PROVIDER is not set (valid values: anthropic, openai)"`
- Error on unknown: `"unknown PI_GO_PROVIDER %q (valid values: anthropic, openai)"`
- Mock provider struct, `NewMockProvider()`, `mockToolCall()` — all deleted from `providers/mock.go`

### Config Changes
- `Default()` still sets `Provider: ""` — but this will fail at runtime if `registerProviders` is called without first loading a provider via env
- `Provider` field comment updated: `// anthropic / openai` (was `// mock / anthropic / openai`)

### Runtime Changes
- `ModelInfo()`: No longer forces `"mock"` when model is empty — returns empty strings
- `buildAgent()`: No longer forces `providerName = "mock"` when model is empty
- `summarizeFunc`: Always uses `compaction.LLMSummarizer` (no mock bypass)

### Test Changes
- `server_test.go`: `newTestApp` sets `cfg.Provider = "openai"`, `cfg.OpenAIAPIKey = "test-key"`, `cfg.OpenAIBaseURL = "http://localhost:4001"`
- `TestServer_Chat`: `t.Skip("skipping: requires a running gateway with valid API key")`
- `builtins_test.go`: All mock session/app data uses `"openai"/"gpt-4o"` instead of `"mock"/"mock"`
- `registry_test.go`: `mockSessionContext.ModelInfo()` returns `"openai", "gpt-4o"`

## Notable Code References

| File | Change |
|------|--------|
| `internal/ai/providers/mock.go` | DELETED (127 lines) |
| `internal/app/app.go` | Mock registration removed, empty provider → error |
| `internal/config/config.go` | Provider comment: `anthropic / openai` |
| `internal/runtime/agent_session.go` | Mock fallback removed from ModelInfo + buildAgent |
| `internal/agents/coding/application.go` | AvailableModels: 5 models (no mock) |
| `internal/server/server_test.go` | newTestApp uses openai, Chat test skipped |
| `internal/agents/coding/commands/builtins_test.go` | mock session/app use openai/gpt-4o |
| `internal/slashcmd/registry_test.go` | mockSessionContext uses openai/gpt-4o |
| `scripts/install_release.sh` | `PI_GO_PROVIDER=openai` |
| `.env.example` | Comment: `anthropic / openai` |

## Contradictions with Existing Wiki

- **[[llm-provider-system]]**: Lists `mock` as a supported provider with ✅ status. **Now removed** — only `anthropic` and `openai` remain. Mock row should be removed entirely (not just marked ❌).
- **[[config-system]]**: Documents `mock` as a valid provider option and shows `Provider` default as `"mock"`. **Mock removed** — default is `""` (empty), and valid values are only `anthropic` / `openai`.
- **[[config-system]]**: Documents the old `registerProviders` fallback behavior. Now errors on empty provider.
- **[[overview.md]]**: Lists "Anthropic / OpenAI / Mock" in capabilities table. **Mock removed** — now "Anthropic / OpenAI".
- **[[coding-application.md]]**: May reference mock in AvailableModels description. Now only lists real providers.
- **[[agent-core]]**: May reference mock in context of agent model resolution. Needs check for mock-related content.
