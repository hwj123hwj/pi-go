export const meta = {
  name: 'dev-pipeline-continue',
  description: 'Pi-Go 后3阶段执行流水线：Execute → Implement → Codex Review（用户审批后使用）',
  whenToUse: 'dev-pipeline 完成设计阶段且用户 approve 后，运行此工作流继续执行。启动方式：/run-workflow dev-pipeline-continue "<主题名>"',
  phases: [
    { title: 'Execute', detail: 'exec-agent 基于已批准提案，产出可施工的 execution-plan.md', model: 'sonnet' },
    { title: 'Implement', detail: 'Claude implement-agent 按执行计划实现代码变更', model: 'sonnet' },
    { title: 'Codex Review', detail: 'Codex review 对代码变更进行审查', model: 'sonnet' },
  ],
}

// ── 常量 ──
const PROJECT_ROOT = '/Users/weijian/Desktop/develop/test/pi/pi-go'
const CODEX_COMPANION = '/Users/weijian/.claude/plugins/cache/openai-codex/codex/1.0.4/scripts/codex-companion.mjs'

// ── Prompt 模板 ──

const EXEC_PROMPT = `You are an **exec-agent** for the pi-go project. Your job: produce a construction-ready execution plan based on the approved proposal and review.

## Prerequisites
- The proposal has been approved
- The review has been approved (address all blockers and strong suggestions)

## Step 1: Read approved documents
- ${PROJECT_ROOT}/docs/dev/{TOPIC}/proposal.md
- ${PROJECT_ROOT}/docs/dev/{TOPIC}/review.md

Pay special attention to the review's blockers (B1, B2, ...) and fix them in the execution plan.
Also address the strong suggestions (S1, S2, ...) where feasible.

## Step 2: Deep-dive into source code details
- Identify exact functions to change, files to create
- Verify each change is feasible at the implementation level
- Read the actual source files the proposal references

## Step 3: Read project context if needed
- ${PROJECT_ROOT}/docs/PROJECT_CONTEXT.md

## Output
Write the execution plan to ${PROJECT_ROOT}/docs/dev/{TOPIC}/execution-plan.md with this YAML frontmatter:

\`\`\`yaml
---
status: approved
author: exec-agent
created: {TODAY}
updated: {TODAY}
depends-on:
  - docs/dev/{TOPIC}/proposal.md
  - docs/dev/{TOPIC}/review.md
---
\`\`\`

The execution plan body MUST include:
1. **整体架构** — architecture diagram + component relationships
2. **这次不做什么** — explicit boundaries
3. **实施步骤** — ordered construction steps, each with:
   - File path to modify/create
   - What exactly to change (function level)
   - How to verify this step is correct
4. **测试策略** — tests to add
5. **迁移注意** — migration path if there are breaking changes

Writing standards:
- Precise down to file and function level — another agent should be able to construct directly from this
- Each step is independently verifiable
- Clear dependencies between steps
- No fluff — every sentence has construction guidance value

## Topic directory
{TOPIC}`

const IMPLEMENT_PROMPT = `You are an **implement-agent** for the pi-go project. Your job: implement the code changes described in the approved execution plan.

## Prerequisites
- The execution plan has been approved

## Step 1: Read the execution plan
- ${PROJECT_ROOT}/docs/dev/{TOPIC}/execution-plan.md

## Step 2: Read project context for coding standards
- ${PROJECT_ROOT}/docs/PROJECT_CONTEXT.md

## Step 3: Implement step by step
Follow the execution plan's ordered construction steps. For each step:
1. Read the existing source file before modifying
2. Make the exact change described (function level)
3. Verify the change is correct (syntax, imports, types)
4. Proceed to next step

## Important
- Do NOT deviate from the execution plan
- Do NOT add scope beyond what's planned
- If you discover something missing or blocked in the plan, stop and report it
- After implementation, run \`go build ./...\` to verify compilation
- All changes should be uncommitted (working tree) so Codex can review the full diff

## Topic directory
{TOPIC}`

const CODEX_REVIEW_PROMPT = `You are a **code review coordinator** for the pi-go project. Your job: delegate code review to Codex and return the result.

## Instructions
1. Run Codex review on the current working tree changes by executing:
   \`\`\`bash
   node "${CODEX_COMPANION}" review --wait
   \`\`\`

2. Capture the full stdout output and return it as your response.

## Important
- Do NOT inspect the code yourself. The review is Codex's job.
- Return the raw Codex review output verbatim.
- If the command fails, report the error message.

## Topic directory
{TOPIC}`

// ── Workflow body ──

const topicName = args || 'untitled-topic'

// Workflow 沙箱禁止 new Date()，通过 args 传入
const today = args?.date || '2026-05-26'

// Helper to fill prompt templates
const fill = (tpl) => tpl
  .replaceAll('{TOPIC}', topicName)
  .replaceAll('{TODAY}', today)

// ── Phase 1: Execute (sonnet: 需要深入源码验证 + 修复 blocker) ──
phase('Execute')
log(`🔧 Exec phase: generating execution plan for "${topicName}"`)
const execResult = await agent(fill(EXEC_PROMPT), {
  label: `exec:${topicName}`,
  phase: 'Execute',
  model: 'sonnet',
})
log(`✅ Execution plan written to docs/dev/${topicName}/execution-plan.md`)

// ── Phase 2: Implement (sonnet: 需要代码理解和精确编辑) ──
phase('Implement')
log(`💻 Implement phase: Claude implementing code for "${topicName}"`)
const implResult = await agent(fill(IMPLEMENT_PROMPT), {
  label: `implement:${topicName}`,
  phase: 'Implement',
  model: 'sonnet',
})
log(`✅ Implementation complete for "${topicName}"`)

// ── Phase 3: Codex Review (sonnet: 执行 codex-companion 转发) ──
phase('Codex Review')
log(`🔍 Codex Review phase: running Codex review on changes`)
const reviewResult = await agent(fill(CODEX_REVIEW_PROMPT), {
  label: `codex-review:${topicName}`,
  phase: 'Codex Review',
  model: 'sonnet',
})
log(`✅ Codex review complete`)

return {
  status: 'done',
  topic: topicName,
  executionPlanPath: `docs/dev/${topicName}/execution-plan.md`,
  implementationSummary: implResult,
  codexReview: reviewResult,
}
