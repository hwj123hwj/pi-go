export const meta = {
  name: 'dev-pipeline',
  description: 'Pi-Go 7阶段完整开发流水线：Research → Plan → Review → Confirm → Execute → Implement → Codex Review',
  whenToUse: '当用户说"开始一个新开发主题"、"走一遍开发流程"、"帮我规划 XXX 功能"、"启动开发流水线"时使用',
  phases: [
    { title: 'Research', detail: 'research-agent 调研参考项目和代码库，产出 research.md', model: 'opus' },
    { title: 'Plan', detail: 'plan-agent 基于调研报告阅读项目上下文，产出提案 proposal.md', model: 'opus' },
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

const RESEARCH_PROMPT = `You are a **research-agent** for the pi-go project (a Go-based Agent framework).

Your job: investigate reference projects and codebases to produce a thorough research report that will feed into the plan phase.

## Step 1: Understand the topic
The topic is: {TOPIC_INFO}

Read the project context to understand pi-go's current state:
- ${PROJECT_ROOT}/docs/PROJECT_CONTEXT.md
- ${PROJECT_ROOT}/docs/PRODUCT_ROADMAP.md

## Step 2: Investigate reference codebases
Based on the topic, identify and read relevant reference implementations. Common reference locations:
- /Users/weijian/Desktop/develop/test/pi/cc-haha/ — Claude Code (TypeScript coding agent)
- /Users/weijian/Desktop/develop/test/pi/codex/ — OpenAI Codex (Rust agent)
- /Users/weijian/Desktop/develop/test/pi/packages/ — Pi monorepo packages

For each reference, focus on:
- How they implement the relevant feature
- Key design patterns and architecture decisions
- Code structure and file organization
- Specific implementation details worth porting

## Step 3: Analyze pi-go's current implementation
Read the relevant pi-go source files to understand the current state and identify gaps.
Focus on the interfaces, types, and extension points that the new feature would touch.

## Step 4: Gap analysis
Compare what pi-go has vs what the reference projects provide. Be specific:
- List concrete missing pieces (not vague descriptions)
- Identify what can be directly ported vs what needs adaptation
- Note any architectural constraints that affect the design

## Output
Write the research report to ${PROJECT_ROOT}/docs/dev/{TOPIC}/research.md with this YAML frontmatter:

\`\`\`yaml
---
status: complete
author: research-agent
created: {TODAY}
updated: {TODAY}
---
\`\`\`

The report body MUST include:
1. **调研目标** — what we're investigating and why
2. **参考项目分析** — for each reference project analyzed:
   - Project name and relevant file paths
   - Key implementation details (with code snippets)
   - Design patterns used
3. **pi-go 现状分析** — current implementation state relevant to the topic
4. **Gap 分析** — concrete differences, table format preferred
5. **移植建议** — what to port, what to adapt, what to skip
6. **关键发现** — important insights that should inform the proposal

Writing standards:
- Include actual file paths from reference projects
- Show relevant code snippets (10-30 lines) to illustrate patterns
- Be specific about interfaces and types, not hand-wavy
- This report will be the primary input for the plan-agent — make it actionable

## Topic
{TOPIC_INFO}`

const PLAN_PROMPT = `You are a **plan-agent** for the pi-go project (a Go-based Agent framework).

Your job: produce a proposal for a new development topic, informed by the research report.

## Context you MUST read first
- ${PROJECT_ROOT}/docs/dev/{TOPIC}/research.md — research report with reference analysis and gap findings
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

// ── Phase 0: Research (opus: 需要深入阅读参考代码库) ──
phase('Research')
log(`🔬 Research phase: investigating reference projects for "${topicName}"`)
const researchResult = await agent(fill(RESEARCH_PROMPT), {
  label: `research:${topicName}`,
  phase: 'Research',
  model: 'opus',
})
log(`✅ Research report written to docs/dev/${topicName}/research.md`)

// ── Phase 1: Plan (opus: 需要深度思考和架构判断) ──
phase('Plan')
log(`📋 Plan phase: generating proposal for "${topicName}" (based on research)`)
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
## 📋 开发流水线产出摘要（设计阶段）

**主题**: ${topicName}

### 🔬 调研报告要点
${researchResult}

---

### 📝 提案核心结论
${planResult}

---

### 🔍 审核结论
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
  researchPath: `docs/dev/${topicName}/research.md`,
  proposalPath: `docs/dev/${topicName}/proposal.md`,
  reviewPath: `docs/dev/${topicName}/review.md`,
  summary,
}
