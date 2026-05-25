# Pi-Go 学习文档

> 本目录记录 Pi-Go 项目的学习笔记和设计分析。

## 文档索引

| 文档 | 内容 |
|---|---|
| [01-agent-framework-extensibility.md](./01-agent-framework-extensibility.md) | 底座架构分析：如何基于通用 Agent 框架扩展不同类型的 Agent |
| [cc-system-prompt.md](./cc-system-prompt.md) | CC 系统提示设计分析：三种构建方式、动态注入机制、与 Pi-Go 的对比 |

## 调研报告

调研报告存放在 `docs/research/` 目录下，使用 `/research` skill 生成。

| 文档 | 内容 |
|---|---|
| [deepvcode-essence-absorption.md](./deepvcode-essence-absorption.md) | DeepVcodeClient vs pi-go 深度对比：功能差距分析与补齐计划 |
| [cc-comparison.md](./cc-comparison.md) | Claude Code 源码分析：插件生态、Hooks 系统与 pi-go 对比借鉴 |
| [oh-my-pi-full-analysis.md](../docs/research/oh-my-pi-full-analysis.md) | oh-my-pi (omp) 全面分析：32 工具/Native Rust/TTSR/Subagent/Hashline 对比 pi-go |
| [competitive-research.md](../docs/research/competitive-research.md) | 竞品调研：阶段性竞品对比分析 |
| [cc-haha-architecture-analysis.md](../docs/research/cc-haha-architecture-analysis.md) | cc-haha 全景分析：桌面工作台/IM/Provider 代理等外围功能对比 |
| [cc-haha-core-engine-analysis.md](../docs/research/cc-haha-core-engine-analysis.md) | **cc-haha 核心引擎源码分析**：Agent 循环/Tool 系统/系统提示/上下文压缩，与 pi-go coding-agent 差距清单 |
| [deepv-code-full-analysis.md](../docs/research/deepv-code-full-analysis.md) | DeepV Code 全面分析：Google Gemini CLI Fork、Proxy 架构、Hook 系统、技能市场，对比 pi-go 迁移建议 |
| [codex-rust-cli-analysis.md](../docs/research/codex-rust-cli-analysis.md) | **OpenAI Codex CLI (Rust) 调研**：90+ crate 架构、Responses API、跨平台沙箱、App Server、MCP 双向集成，对比 pi-go 迁移建议 |

## 设计文档

| 文档 | 内容 |
|---|---|
| [coding-agent-spec.md](../docs/dev/coding-agent/spec.md) | Coding Agent 实现规格：目录结构、接口定义、实现要求 |
