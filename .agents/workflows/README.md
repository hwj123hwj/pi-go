# 开发流水线 Workflows

pi-go 项目采用 7 阶段开发流水线，分为**设计阶段**（Research → Plan → Review → Confirm）和**执行阶段**（Execute → Implement → Codex Review）。

---

## 使用方式

### 1. 启动设计阶段

```
/run-workflow dev-pipeline "<主题描述>"
```

自动执行：
```
Phase 0: Research (opus)  → research.md    调研参考项目（cc/codex 等）
Phase 1: Plan    (opus)   → proposal.md    基于调研产出提案
Phase 2: Review  (opus)   → review.md      独立审核 + 源码交叉验证
Phase 3: Confirm           → 展示摘要，等你审批
```

### 2. 审批后继续执行

审批通过后：
```
/run-workflow dev-pipeline-continue "<主题名>"
```

自动执行：
```
Phase 4: Execute       (sonnet) → execution-plan.md
Phase 5: Implement     (sonnet) → Claude 按计划写代码
Phase 6: Codex Review  (sonnet) → Codex 审查代码 diff
```

---

## 快速参考

| 阶段 | 阶段名 | 模型 | 产出 |
|------|--------|------|------|
| 设计 | Research | opus | research.md |
| 设计 | Plan | opus | proposal.md |
| 设计 | Review | opus | review.md |
| 设计 | Confirm | — | 等待用户审批 |
| 执行 | Execute | sonnet | execution-plan.md |
| 执行 | Implement | sonnet | 代码变更（working tree） |
| 执行 | Codex Review | sonnet | Codex 审查报告 |

> 产出文档在 `docs/dev/<主题>/` 目录下。

---

## Research 阶段说明

Research 是流水线的起点，解决"Plan agent 对参考代码库不够了解"的问题：

- **调研 cc (Claude Code)** — TypeScript 编码 Agent 的工具实现、多 agent 协调、并行操作
- **调研 codex** — Rust Agent 的工具基础设施、并行执行引擎
- **调研 pi monorepo** — 上游项目的共享包设计
- **产出 research.md** — 包含代码片段、gap 分析、移植建议

Plan 阶段依赖 research.md 作为输入，提案质量大幅提升。
