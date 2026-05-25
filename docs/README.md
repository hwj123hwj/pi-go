# Pi-Go 文档中心

> 本目录是 pi-go 项目的所有文档入口。按文档性质分目录组织，方便不同角色（调研/计划/审核/执行 agent）快速定位。

---

## 目录结构

```
docs/
├── README.md                          ← 你在这里
├── PROJECT_CONTEXT.md                 # 项目上下文快照
├── PRODUCT_ROADMAP.md                 # 产品路线图
├── CONTRIBUTING.md                    # 贡献指南
├── deploy.md                          # 部署说明
│
├── references/                        # 参考资料（不指导开发，只供查阅）
│   ├── feishu-integration-ref.md      # 飞书接入参考
│   └── skills-vs-application.md       # Skills 与 Application 扩展路径决策参考
│
├── research/                          # 外部项目调研报告（调研 agent 产出）
│   ├── oh-my-pi-full-analysis.md
│   ├── cc-haha-architecture-analysis.md
│   ├── cc-haha-core-engine-analysis.md
│   ├── codex-rust-cli-analysis.md
│   ├── deepv-code-full-analysis.md
│   └── competitive-research.md        # (archived)
│
├── dev/                               # 开发文档（4-agent 流水线产出）
│   ├── layering-refactor/             # 分层重构
│   ├── coding-agent/                  # Coding Agent 规格
│   ├── coding-agent-cli-control-plane/# Coding Agent CLI 控制面
│   ├── coding-agent-slash-hardening/  # Coding Agent Slash 第二阶段补强
│   └── second-agent-validation/       # 第二个 Agent 验证（暂缓预案）
│
└── archive/                           # 已完成的开发主题
    ├── cli-tui/
    ├── ssh-operations/
    ├── tool-lifecycle/
    ├── desktop-golang/
    ├── project-overview.md
    └── code-review-issues.md
```

---

## 长期文档

这些文档始终位于 `docs/` 根目录，持续维护：

| 文档 | 用途 |
|------|------|
| [PROJECT_CONTEXT.md](PROJECT_CONTEXT.md) | 项目架构、核心能力、技术栈快照。调研 agent 和维护 agent 的对比基准 |
| [PRODUCT_ROADMAP.md](PRODUCT_ROADMAP.md) | 产品与技术主路线图 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 贡献和开发协作说明 |
| [deploy.md](deploy.md) | 自动部署说明 |

## 参考资料

`references/` 下是不直接指导开发、仅供查阅的参考资料。

| 文档 | 用途 |
|------|------|
| [feishu-integration-ref.md](references/feishu-integration-ref.md) | 飞书 Bot 接入参考 |
| [skills-vs-application.md](references/skills-vs-application.md) | Skills 与 Application 扩展路径决策参考 |
| [pi-go-analysis.md](references/pi-go-analysis.md) | pi-go 项目深度分析报告（架构评估、Bug 清单、功能差距） |

## 调研报告

`research/` 下是调研 agent 对外部项目的分析报告。

| 文档 | 内容 |
|------|------|
| [oh-my-pi-full-analysis.md](research/oh-my-pi-full-analysis.md) | oh-my-pi (omp) 全面分析：32 工具、Native Rust、TTSR、Subagent 等 |
| [cc-haha-architecture-analysis.md](research/cc-haha-architecture-analysis.md) | cc-haha 桌面端架构与功能全景分析 |
| [cc-haha-core-engine-analysis.md](research/cc-haha-core-engine-analysis.md) | cc-haha Agent 循环、Tool 系统、核心引擎源码深度分析 |
| [codex-rust-cli-analysis.md](research/codex-rust-cli-analysis.md) | Codex Rust CLI 详细调研 |
| [deepv-code-full-analysis.md](research/deepv-code-full-analysis.md) | DeepV Code 全面分析 |
| [manual-compaction-design-analysis.md](research/manual-compaction-design-analysis.md) | `/compact` 手动上下文治理设计收敛文档 |
| ~~competitive-research.md~~ | *(已归档)* 阶段性竞品对比分析 |

## 开发文档

`dev/` 下按主题分目录，每个目录对应一个开发主题，包含该主题的全生命周期文档：

```
dev/{topic}/
├── proposal.md          # 提案（plan-agent 产出）
├── review.md            # 审核（review-agent 产出）
└── execution-plan.md    # 执行计划（确认后，exec-agent 使用）
```

每个 `dev/` 文档都有 YAML 元信息头，标注状态和角色：

```yaml
---
status: draft | reviewed | approved | done
author: plan-agent | review-agent | exec-agent | research-agent
created: YYYY-MM-DD
updated: YYYY-MM-DD
---
```

**状态流转**：`draft → reviewed → approved → done`（rejected 回到 draft）

| 主题 | 状态 | 文档 |
|------|------|------|
| 分层重构 | approved | [proposal.md](dev/layering-refactor/proposal.md) |
| Coding Agent | approved | [spec.md](dev/coding-agent/spec.md) |
| Coding Agent CLI 控制面 | done | [execution-plan.md](dev/coding-agent-cli-control-plane/execution-plan.md) |
| Coding Agent Slash Hardening | approved | [execution-plan.md](dev/coding-agent-slash-hardening/execution-plan.md) |
| 第二个 Agent 验证 | deferred | [execution-plan.md](dev/second-agent-validation/execution-plan.md) |

## 归档

`archive/` 下是已完成的开发主题。整个主题目录从 `dev/` 移入，保留完整上下文。

| 归档主题 | 内容 |
|---------|------|
| [cli-tui/](archive/cli-tui/execution-plan.md) | CLI/TUI 执行计划（已完成，产品方向已明确为 CLI） |
| [runtime-decoupling/](archive/runtime-decoupling/execution-plan.md) | Runtime 解耦执行计划（已完成） |
| [ssh-operations/](archive/ssh-operations/execution-plan.md) | SSH 远程执行计划（已完成） |
| [tool-lifecycle/](archive/tool-lifecycle/execution-plan.md) | Tool Lifecycle 执行计划（已完成） |
| [desktop-golang/](archive/desktop-golang/changes.md) | Desktop Go 实现变更说明 |
| [project-overview.md](archive/project-overview.md) | 项目概览（历史快照） |
| [code-review-issues.md](archive/code-review-issues.md) | 代码评审问题清单 |

---

## 维护规则

- **docs-maintainer skill** 负责定期同步文档与代码的一致性
- **research skill** 负责生成 `research/` 下的调研报告
- 新开发主题由 plan-agent 在 `dev/` 下创建目录
- 完成的主题整体移入 `archive/`
