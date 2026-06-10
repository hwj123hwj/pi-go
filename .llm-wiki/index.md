# LLM Wiki Index

> Auto-maintained by Easy Code. Do not edit manually.

## Sources
- [[source-project-root]] — Full ingest of project root (.)

## Entities
- [[agent-core]] — Agent state machine and execution engine
- [[tool-system]] — 7 built-in tools + optional interfaces
- [[llm-provider-system]] — Anthropic/OpenAI/DeepV/Mock providers
- [[session-persistence]] — JSONL append-only with tree branching
- [[slash-command-framework]] — 14 built-in slash commands
- [[tool-lifecycle-hooks]] — Before/After hooks and argument preparation
- [[extension-system]] — Plugin-style tools/commands/hooks
- [[skill-system]] — Markdown skill loading from `.claude/skills/`

## Concepts
- [[four-layer-architecture]] — Entrypoints → Application → Platform → Core
- [[runtime-application-interface]] — The Platform↔App decoupling boundary
- [[agent-loop]] — Outer follow-up + inner tool-call dual loop
- [[context-compaction]] — LLM-driven conversation summarization
- [[goal-driven-loop]] — Autonomous goal-directed agent execution
- [[operations-abstract]] — Local/SSH execution backend switching

## Synthesis
<!-- Cross-cutting analysis pages will be listed here -->