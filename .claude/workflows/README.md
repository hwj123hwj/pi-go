# 开发流水线 Workflows

pi-go 项目采用 6 阶段开发流水线，分为**设计阶段**（Plan → Review → Confirm）和**执行阶段**（Execute → Implement → Codex Review）。

---

## 使用方式

### 1. 启动设计阶段

```
/run-workflow dev-pipeline "<主题描述>"
```

自动执行：
```
Phase 1: Plan    (opus)  → proposal.md
Phase 2: Review  (opus)  → review.md
Phase 3: Confirm         → 展示摘要，等你审批
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
| 设计 | Plan | opus | proposal.md |
| 设计 | Review | opus | review.md |
| 设计 | Confirm | — | 等待用户审批 |
| 执行 | Execute | sonnet | execution-plan.md |
| 执行 | Implement | sonnet | 代码变更（working tree） |
| 执行 | Codex Review | sonnet | Codex 审查报告 |

> 产出文档在 `docs/dev/<主题>/` 目录下。
