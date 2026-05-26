export const meta = {
  name: 'dev-pipeline',
  description: 'Pi-Go 6阶段完整开发流水线：Plan → Review → Confirm → Execute → Implement → Codex Review',
  whenToUse: '当用户说"开始一个新开发主题"、"走一遍开发流程"、"帮我规划 XXX 功能"、"启动开发流水线"时使用',
  phases: [
    { title: 'Plan', detail: 'plan-agent 阅读项目上下文，产出提案 proposal.md', model: 'opus' },
    { title: 'Review', detail: 'review-agent 独立审核提案，交叉验证源码，产出 review.md', model: 'opus' },
    { title: 'Confirm', detail: '展示提案摘要和审核结论，等待用户 approve/reject' },
    { title: 'Execute', detail: 'exec-agent 基于已批准提案，产出可施工的 execution-plan.md', model: 'sonnet' },
    { title: 'Implement', detail: 'Claude implement-agent 按执行计划实现代码变更', model: 'sonnet' },
    { title: 'Codex Review', detail: 'Codex review 对代码变更进行审查', model: 'sonnet' },
  ],
}

// ── 常量 ──
// Workflow 沙箱没有 process.cwd() / new Date()，必须硬编码
const PROJECT_ROOT = '/Users/weijian/Desktop/develop/test/pi/pi-go'
const CODEX_COMPANION = '/Users/weijian/.claude/plugins/cache/openai-codex/codex/1.0.4/scripts/codex-companion.mjs'

// ── Prompt 模板 ──

const PLAN_PROMPT = `You are a **plan-agent** for the pi-go project (a Go-based Agent framework).

Your job: produce a proposal for a new development topic.

## Context you MUST read first
- ${PROJECT_ROOT}/docs/PROJECT_CONTEXT.md — current architecture, core capabilities, key interfaces
- ${PROJECT_ROOT}/docs/PRODUCT_ROADMAP.md — product roadmap, understand where this topic fits
- ${PROJECT_ROOT}/docs/README.md — existing dev topics, avoid duplication

Then read relevant references based on the topic:
- Files in ${PROJECT_ROOT}/docs/decisions/ — relevant design decisions
- Files in ${PROJECT_ROOT}/docs/research/ — relevant research reports
- Related source code files (locate based on the topic)

## Output
Write the proposal to ${PROJECT_ROOT}/docs/dev/{TOPIC}/proposal.md with this YAML frontmatter:

\`\`\`yaml
---
status: draft
author: plan-agent
created: {TODAY}
updated: {TODAY}
---
\`\`\`

The proposal body MUST include these sections:
1. **目标** — one-sentence goal
2. **为什么现在做** — motivation and timing
3. **这次做什么** — specific scope
4. **这次不做什么** — explicit boundaries
5. **技术方案** — architecture impact, core design (interface changes, new types, data flow), key files to modify/create
6. **依赖关系** — dependencies on existing capabilities or unfinished work
7. **风险和取舍** — potential pitfalls, intentional exclusions
8. **完成标志** — how to know it's done

Writing standards:
- Reference actual file paths from the codebase
- Give clear technical judgments, no fence-sitting
- If multiple approaches exist, recommend one with reasons
- Keep it focused, longer is not better

## Topic
{TOPIC_INFO}`

const REVIEW_PROMPT = `You are a **review-agent** for the pi-go project. Your job: independently review the plan-agent's proposal.

## Step 1: Read the proposal
Read ${PROJECT_ROOT}/docs/dev/{TOPIC}/proposal.md

## Step 2: Cross-validate against source code
For every key file, interface, or function the proposal mentions, verify it actually exists in the source code. Check for missing dependencies or conflicts the proposal might have overlooked.

## Step 3: Review along these dimensions
| Dimension | Focus |
|-----------|-------|
| Accuracy | Does the proposal correctly describe the current state of the code? |
| Completeness | Are there missing impact areas (other modules, config, tests)? |
| Feasibility | Can the technical plan actually be implemented? Any showstoppers? |
| Consistency | Does it align with existing architecture decisions and layering rules? |
| Risk blindspots | Is complexity underestimated or risks missed? |
| Scope control | Is the scope reasonable, or is it secretly expanding? |

## Output
Write the review to ${PROJECT_ROOT}/docs/dev/{TOPIC}/review.md with this YAML frontmatter:

\`\`\`yaml
---
status: reviewed
author: review-agent
created: {TODAY}
updated: {TODAY}
reviewer: review-agent
review-status: pending
depends-on:
  - docs/dev/{TOPIC}/proposal.md
---
\`\`\`

The review body MUST include:
1. **总体评价** — one of: approve / needs-revision / reject
2. **准确性验证** — verify each key claim in the proposal against source code
3. **发现的问题** — prioritized:
   - 🔴 Blockers (must fix)
   - 🟡 Strong suggestions
   - 🟢 Nice-to-haves
4. **遗漏检查** — things the proposal might have missed
5. **修改建议** — concrete suggestions for each issue

## Topic directory
{TOPIC}`

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

const topicInfo = args || 'untitled-topic'

// Extract topic name (first word before any dash/description)
const topicName = topicInfo.split(/\s*[—\-–]\s*/)[0].trim().replace(/\s+/g, '-')

// Workflow 沙箱禁止 new Date()，通过 args 传入
const today = args?.date || '2026-05-26'

// Helper to fill prompt templates
const fill = (tpl) => tpl
  .replaceAll('{TOPIC_INFO}', topicInfo)
  .replaceAll('{TOPIC}', topicName)
  .replaceAll('{TODAY}', today)

// ── Phase 1: Plan (opus: 需要深度思考和架构判断) ──
phase('Plan')
log(`📋 Plan phase: generating proposal for "${topicName}"`)
const planResult = await agent(fill(PLAN_PROMPT), {
  label: `plan:${topicName}`,
  phase: 'Plan',
  model: 'opus',
})
log(`✅ Proposal written to docs/dev/${topicName}/proposal.md`)

// ── Phase 2: Review (opus: 需要独立批判性思维和交叉验证) ──
phase('Review')
log(`🔍 Review phase: independent review of proposal`)
const reviewResult = await agent(fill(REVIEW_PROMPT), {
  label: `review:${topicName}`,
  phase: 'Review',
  model: 'opus',
})
log(`✅ Review written to docs/dev/${topicName}/review.md`)

// ── Phase 3: Confirm (user gate — 展示摘要，等待人工审批) ──
phase('Confirm')
const summary = `
## 📋 开发流水线产出摘要

**主题**: ${topicName}

### 提案核心结论
${planResult}

---

### 审核结论
${reviewResult}

---

**下一步**: 请确认是否继续执行阶段？
- 回复 **approve** → 继续产出 execution-plan.md → 实现 → Codex 审查
- 回复 **needs-revision** → 修改提案后重新审核
- 回复 **reject** → 放弃此主题
`

log(summary)

return {
  status: 'awaiting-approval',
  topic: topicName,
  proposalPath: `docs/dev/${topicName}/proposal.md`,
  reviewPath: `docs/dev/${topicName}/review.md`,
  summary,
}
