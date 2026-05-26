# DeepVcodeClient 精要吸收报告 — pi-go 增强建议

> 基于对 pi-go（pi 的 Go 重写）与 DeepVcodeClient（商业化 Coding Agent）的深度对比分析。
> 目标：识别 pi-go coding-agent 层的功能差距，制定补齐计划。

---

## 目录

1. [概述](#1-概述)
2. [高优先级 —— 核心缺失功能](#2-高优先级--核心缺失功能)
3. [中优先级 —— 质量与体验提升](#3-中优先级--质量与体验提升)
4. [低优先级 —— 锦上添花](#4-低优先级--锦上添花)
5. [架构改进建议](#5-架构改进建议)
6. [按模块的详细对比表](#6-按模块的详细对比表)
7. [实施路线图建议](#7-实施路线图建议)

---

## 1. 概述

### 项目关系

| 项目 | 角色 | 特点 |
|------|------|------|
| pi-go（Go） | 我们的重写 | pi 的 Go 实现，通用 Agent 底座 + coding-agent 应用层 |
| DeepVcodeClient（TypeScript） | 商业化成熟产品 | 大量工程化、安全、UX 增强特性，作为功能对标的参考 |

### pi-go 当前已完成的核心能力

- ✅ 统一 LLM API 抽象（ai 层 + 多 Provider：anthropic/openai/mock）
- ✅ Agent 双层循环（外层 follow-up + 内层 tool call）
- ✅ 7 个内置工具（read/write/edit/bash/grep/find/ls）
- ✅ 工具可选接口（`ToolWithMode` 控制并行/串行、`ToolWithPromptInfo` 注入系统提示片段）
- ✅ 工具过滤机制（`AllowedTools` / `BlockedTools` 配置）
- ✅ 事件系统 + 流式输出（SSE）
- ✅ 上下文压缩（Compaction：LLM 摘要旧上下文 + 保留最近消息）
- ✅ 会话持久化（JSONL 树状存储）
- ✅ 扩展系统（Extensions）
- ✅ 技能系统（Skills）
- ✅ HTTP API Server（/health、/chat、/chat/stream、/sessions 等）
- ✅ 多种执行模式（interactive/print/serve）

### pi-go 当前的安全限制

- Bash 工具默认**禁用**（`EnableBash: false`），需显式开启
- 工具路径安全检查函数已实现（`IsPathSafe()`），但尚未全面集成到所有工具执行流程中
- 没有危险命令检测、没有执行确认机制、没有循环检测

### DeepVcodeClient 领先的关键方向

DeepVcodeClient 作为一个**商业化产品**，在以下方面远超 pi-go：

1. **工具数量与质量**：30+ 工具 vs 7 个工具，且每个工具都有参数校验、确认机制、结果渲染
2. **安全与权限**：危险命令检测、文件安全路径检查、用户确认流程（5 种确认类型 + 6 种确认结果）
3. **上下文管理**：三级压缩策略（全量 LLM 压缩 + MicroCompact + PostCompactRestoration）+ 循环检测
4. **IDE 集成**：LSP 代码智能、VSCode 插件、lint 工具
5. **Agent 协作**：SubAgent 系统、Task 工具、任务委派
6. **质量保障**：自动 lint 检查、编辑纠错、Diff 渲染
7. **UX 体验**：多 Agent 风格（Claude/Cursor/Codex/Augment/Windsurf）、Todo 管理

---

## 2. 高优先级 —— 核心缺失功能

> 这些功能能显著提升 pi-go 的 coding-agent 能力，且实现路径清晰。

### 2.1 更完善的工具集

#### 2.1.1 ReadManyFilesTool
- **参考**: `DeepVcodeClient/packages/core/src/tools/read-many-files.ts`
- **价值**: 批量读取文件，支持 glob 模式匹配、自动排除规则
- **关键特性**:
  - `paths: string[]` 批量路径
  - Glob 模式匹配
  - 智能内容截断（总字符预算）
  - `.gitignore` / `.geminiignore` 尊重
  - 文件类型感知（binary vs text）
- **实现建议**:
```go
type ReadManyFilesParams struct {
    Paths       []string `json:"paths"`
    Exclude     []string `json:"exclude,omitempty"`
    MaxChars    int      `json:"max_chars,omitempty"` // 总字符预算
}
```

#### 2.1.2 AskUserQuestionTool
- **参考**: `DeepVcodeClient/packages/core/src/tools/ask-user-question.ts`
- **价值**: Agent 可以在执行过程中向用户提问确认
- **关键特性**:
  - 多选/单选问题
  - 问题+选项+描述 结构
  - 支持"其他"自定义输入
- **实现建议**:

```go
type AskUserQuestionParams struct {
    Questions []Question `json:"questions"`
}

type Question struct {
    Question    string      `json:"question"`
    Header      string      `json:"header"`
    Options     []Option    `json:"options"`
    MultiSelect bool        `json:"multi_select,omitempty"`
}

type Option struct {
    Label       string `json:"label"`
    Description string `json:"description"`
}
```

#### 2.1.3 TaskTool / SubAgentTool
- **参考**: `DeepVcodeClient/packages/core/src/tools/task.ts` + `core/subAgent.ts`
- **价值**: Agent 可以创建子 Agent 来处理复杂子任务
- **关键特性**:
  - 启动一个子 Agent 会话
  - 子 Agent 共享父 Agent 的工具子集
  - 结果返回父 Agent 继续处理
  - 并发启动多个 Task 进行并行探索
- **实现建议**: 可以利用 pi-go 的 `runtime.AgentSession` 机制，新建一个 session 作为 sub-agent

#### 2.1.4 MemoryTool
- **参考**: `DeepVcodeClient/packages/core/src/tools/memoryTool.ts`
- **价值**: Agent 可以记住跨会话的用户偏好和信息
- **关键特性**:
  - 存储 key-value 对到文件
  - 读取所有记忆
  - 支持 GEMINI.md / CLAUDE.md 格式

#### 2.1.5 DeleteFileTool
- **参考**: `DeepVcodeClient/packages/core/src/tools/delete-file.ts`
- **价值**: 安全的文件删除操作
- **关键特性**:
  - 确认流程（文件内容预览）
  - 不能删除工作目录之外的文件

#### 2.1.6 增强 FindTool（增加 glob 和 .gitignore 支持）

> ⚠️ 不新增 GlobTool，而是增强现有的 FindTool。pi-go 已有 `find.go`（支持 glob 和正则），功能与 DeepVcodeClient 的 GlobTool 重叠，没必要两个工具。

- **参考**: `DeepVcodeClient/packages/core/src/tools/glob.ts`
- **价值**: 增强 FindTool 使其更实用
- **需要补充的**:
  - `.gitignore` 规则支持（跳过被 gitignore 的文件）
  - 更好的 glob 模式匹配（当前 `globToRegex` 实现较简单）
  - 结果排序和截断

#### 2.1.7 LintReadTool + LintFixTool
- **参考**: `DeepVcodeClient/packages/core/src/tools/read-lints.ts` + `lint-fix.ts`
- **价值**: 代码编辑后自动检查代码质量
- **特性**:
  - 读取当前文件的 linter 诊断结果
  - 自动修复常见的 lint 问题
  - 与编辑器/语言服务器集成

### 2.2 工具执行确认机制（安全层）

DeepVcodeClient 的每个 Tool 都有 `shouldConfirmExecute()` 方法，对危险操作进行用户确认。

#### 参考文件
- `DeepVcodeClient/packages/core/src/tools/tools.ts` — `ToolCallConfirmationDetails` 联合类型（5 种确认类型：edit/exec/delete/info/mcp）
- `DeepVcodeClient/packages/core/src/config/config.ts` — `ApprovalMode` 配置（6 种确认结果：ProceedOnce/ProceedAlways/ProceedAlwaysServer/ProceedAlwaysTool/ProceedAlwaysProject/Cancel）

#### 实现建议

pi-go 当前已有两个可选接口（`ToolWithMode` 和 `ToolWithPromptInfo`），在此基础上新增确认接口：

```go
// 已有的可选接口（internal/agent/tool.go）
type ToolWithMode interface {
    Tool
    Mode() ExecutionMode // parallel/sequential
}

type ToolWithPromptInfo interface {
    Tool
    PromptInfo() PromptSnippet
}

// 新增：确认接口
type ToolWithConfirmation interface {
    Tool
    // ShouldConfirm 返回是否需要用户确认，以及确认信息
    ShouldConfirm(params json.RawMessage) (*ConfirmationInfo, error)
}

type ConfirmationInfo struct {
    Type    ConfirmationType // "edit" | "exec" | "delete" | "info"
    Title   string
    Prompt  string           // 给用户看的具体描述
    Details *ConfirmationDetails
}

type ConfirmationDetails struct {
    FilePath        string // 受影响的文件
    Command         string // 要执行的命令
    OriginalContent string // 原始内容（编辑操作）
    NewContent      string // 新内容（编辑操作）
}
```

Agent 循环中需要增加确认等待机制：

```go
func executeOneTool(ctx context.Context, a *Agent, call ai.ToolCall) ai.Message {
    // ... 现有逻辑 ...

    // 新增：检查是否需要确认
    if cf, ok := tool.(ToolWithConfirmation); ok {
        info, err := cf.ShouldConfirm(validated)
        if err == nil && info != nil {
            // 发送确认事件给上层
            a.emit(ctx, EventAwaitingConfirmation{
                ToolCallID: call.ID,
                ToolName:   call.Name,
                Confirmation: info,
            })
            // 等待用户确认（通过 channel，带超时）
            confirmed := a.waitForConfirmation(ctx, call.ID)
            if !confirmed {
                return ai.ToolResultMessage{
                    ToolCallID: call.ID,
                    Content:    "Execution cancelled by user",
                    IsError:    true,
                }
            }
        }
    }

    // ... 继续执行 ...
}
```

#### 不同模式下的确认传输

确认机制需要根据执行模式做适配：

| 模式 | 确认方式 | 实现 |
|------|----------|------|
| **interactive（交互）** | TUI 中渲染确认提示，等待用户输入 y/n | 直接在终端输出确认信息，通过 stdin 读取 |
| **serve（HTTP API）** | SSE 新增 `awaiting_confirmation` 事件类型 + `/chat/:id/confirm` 端点 | 前端收到事件后弹出确认对话框，调用 confirm 端点 |
| **print（单次）** | 默认自动确认（非交互模式没有用户参与） | 通过 `--auto-confirm` flag 控制是否跳过 |

### 2.3 循环检测（Loop Detection）

DeepVcodeClient 拥有成熟的循环检测系统，防止 Agent 陷入无限循环浪费 API 配额。

#### 参考文件
- `DeepVcodeClient/packages/core/src/services/loopDetectionService.ts`

#### 检测策略

| 检测类型 | 阈值 | 说明 |
|----------|------|------|
| 连续相同 tool call | 10 次 | 完全相同参数的 tool call 重复 |
| 相同工具名称循环（预览模型） | 32 次 | 相同工具名但参数不同 |
| 重复文本输出 | 20 次 | 输出相同的 text chunk |
| LLM 辅助检测 | 30 轮后 | 让 LLM 自己判断是否在循环 |

#### 实现建议

```go
type LoopDetector struct {
    mu                sync.Mutex
    recentToolCalls   []ToolCallRecord
    recentTexts       []string
    consecutiveSameTool int
    promptID          string
}

type ToolCallRecord struct {
    Name      string
    ArgsHash  string // 参数哈希
    Timestamp time.Time
}

func (d *LoopDetector) Check(call ai.ToolCall) LoopResult {
    // 1. 检测相同 tool + 相同参数
    // 2. 检测相同 tool 名重复
    // 3. 检测文本输出循环
}
```

### 2.4 MicroCompact（微压缩）

DeepVcodeClient 在常规全量 LLM 压缩之外，还实现了轻量级微压缩。

#### 参考文件
- `DeepVcodeClient/packages/core/src/services/microCompactService.ts`

#### 关键策略
- **空闲超时**：距上次助手消息超过 60 分钟，清除旧工具结果占位符
- **保留最近 N 个**：保留最近的 5 个工具结果不被清除
- **Token 缓冲**：当上下文使用率达到 70% 时触发（全量压缩是 80%）
- **可压缩工具集**：只清理 `read_file`、`run_shell_command` 等大型输出工具

#### 实现建议

```go
type MicroCompactService struct {
    idleThreshold      time.Duration // 60分钟
    keepRecent         int           // 5
    tokenThreshold     float64       // 0.7
    lastAssistantAt    time.Time
    compressibleTools  []string      // 只有这些工具的输出可以被替换为占位符
}

// 注意：不是所有工具输出都能清除。edit 的结果不能丢，
// 只有 read_file、run_shell_command 等大型输出工具的输出可以替换。
// DeepVcodeClient 明确限制了可压缩工具集。
```

### 2.5 PostCompactRestoration（压缩后文件恢复）

#### 参考文件
- `DeepVcodeClient/packages/core/src/services/postCompactRestorationService.ts`

#### 价值
压缩后最近读取的文件内容丢失，AI 需要重新读取。PostCompactRestoration 自动追踪最近读取的文件，在压缩后自动附加其内容到对话上下文。

#### 实现建议

```go
type PostCompactRestoration struct {
    mu             sync.Mutex
    recentReads    []string // 最近读取的文件路径
    maxFiles       int      // 最多恢复5个
    maxCharsPerFile int     // 每个文件最多5000字符
    totalBudget    int      // 总共50000字符
}

func (r *PostCompactRestoration) TrackFileRead(path string) { ... }
func (r *PostCompactRestoration) GenerateRestorationContent() string { ... }
```

### 2.6 Edit Corrector（编辑纠错）—— 提自中优先级

> ⚠️ 从原 §3.6 移入。Edit Corrector 解决的是 EditTool `old_string` 匹配失败的问题——这是 Agent 实际使用中**最高频的失败原因**，应作为高优先级实现。

#### 参考文件
- `DeepVcodeClient/packages/core/src/utils/editCorrector.ts`

#### 价值
当 `EditTool` 的 `old_string` 匹配失败时，自动计算最近距离编辑建议。不解决此问题，Agent 在编辑代码时会频繁失败并浪费 turn 重试。

#### 实现建议

```go
// EditCorrector 当 old_string 找不到时，建议最接近的匹配
type EditCorrector struct{}

type CorrectionSuggestion struct {
    LineStart int
    LineEnd   int
    Content   string
    Score     float64 // 相似度评分
}

func (c *EditCorrector) SuggestCorrection(content, oldString string) []CorrectionSuggestion {
    // 1. 按行查找最接近的匹配（基于 Levenshtein 距离）
    // 2. 考虑缩进差异、空行差异
    // 3. 返回前 3 个建议
}

// 在 EditTool.Execute 中，当 old_string 匹配失败时调用：
// suggestions := corrector.SuggestCorrection(fileContent, params.OldString)
// 将建议附加到错误信息中，帮助 LLM 在下一轮自动修正
```

---

## 3. 中优先级 —— 质量与体验提升

> ⚠️ Edit Corrector（编辑纠错）已移至 §2.6 高优先级。

### 3.1 LSP 代码智能工具集

#### 参考
- `DeepVcodeClient/packages/core/src/tools/lsp/` 目录

DeepVcodeClient 有 6 个 LSP 工具，让 Agent 像 IDE 一样理解代码：

| 工具 | 功能 | 使用场景 |
|------|------|----------|
| `lsp_hover` | 查看类型信息 | "这个函数返回什么类型？" |
| `lsp_goto_definition` | 跳转到定义 | "XXX 在哪里定义的？" |
| `lsp_find_references` | 查找引用 | "这个函数在哪里被调用？" |
| `lsp_document_symbols` | 文件内符号列表 | "这个文件有哪些函数/类？" |
| `lsp_workspace_symbols` | 全局符号搜索 | "项目中所有 UserService 相关" |
| `lsp_implementation` | 接口实现查找 | "这个接口有哪些实现？" |

#### 实现建议
需要集成 LSP 客户端（Go 生态有 `go.lsp.dev` / `sourcegraph/go-lsp`），在 Agent 启动时启动语言服务器。每个 LSP 工具本质上是向 LSP Server 发送 JSON-RPC 请求。

### 3.2 多 Agent 风格（Style System）

#### 参考
- `DeepVcodeClient/packages/core/src/core/prompts.ts` — `getStaticSystemPrompt()`

DeepVcodeClient 支持 6 种 Agent 风格，每种是完全不同的系统提示：

| 风格 | 特点 | 场景 |
|------|------|------|
| `default` | Claude Code 风格，平衡 | 通用 |
| `codex` | 极简、无声执行、输出少 | 自动化任务 |
| `cursor` | 强调语义搜索、并行工具 | 代码导航 |
| `augment` | 任务列表驱动、严格验证 | 复杂重构 |
| `windsurf` | AI Flow 风格 | 长任务 |
| `antigravity` | 美学导向、知识发现 | 前端开发 |

### 3.3 系统提示的 Prompt Cache 边界标记

#### 参考
- `DeepVcodeClient/packages/core/src/core/prompts.ts:42` — `SYSTEM_PROMPT_DYNAMIC_BOUNDARY`

系统提示分为静态部分（所有用户相同，适合 prompt cache）和动态部分（用户特定）。

#### 实现建议

```go
const SystemPromptDynamicBoundary = "__SYSTEM_PROMPT_DYNAMIC_BOUNDARY__"

func BuildSystemPrompt(opts Options) string {
    var b strings.Builder

    // 静态部分（不变的内容，Provider 可以缓存）
    b.WriteString(coreBehaviorSection)
    b.WriteString(toolSummarySection(opts.Tools))
    b.WriteString(guidelinesSection)

    // 边界标记
    b.WriteString(SystemPromptDynamicBoundary)

    // 动态部分（每次不同的内容，不需要缓存）
    b.WriteString(projectContextSection(opts.CWD, opts.ContextFiles))
    b.WriteString(skillsSection(opts.Skills))
    b.WriteString(runtimeInfoSection(opts.CWD))

    return b.String()
}
```

对于支持 Prompt Cache 的 Provider（如 Anthropic），可以在 API 调用时将边界前的内容标记为可缓存，减少重复传输成本。

### 3.4 危险命令检测

#### 参考
- `DeepVcodeClient/packages/core/src/utils/dangerous-command-detector.ts`

检测并标记危险命令（rm -rf、dd、format 等），自动请求用户确认。

### 3.5 Shell 命令安全增强

#### 参考
- `DeepVcodeClient/packages/core/src/utils/shell-utils.ts`

- 命令白名单机制（`isCommandAllowed()`）
- 命令根命令提取（`getCommandRoots()`）
- 包装命令剥离（`stripShellWrapper()`）
- Windows 编码自动检测（中文系统自动用 GBK 编码）

### 3.6 自动 Lint 检查（编辑后）

#### 参考
- `DeepVcodeClient/packages/core/src/tools/helpers/autoLintChecker.ts`

编辑完成后自动跑 lint，将结果返回给 Agent：

```go
// 在 EditTool.Execute 末尾
lintResult := runLinter(ctx, filePath)
if lintResult.HasIssues {
    return ToolResult{
        Content: editResult + "\n\n" + lintResult.Summary,
    }
}
```

### 3.7 TodoWrite 工具

#### 参考
- `DeepVcodeClient/packages/core/src/tools/todo-write.ts`

Agent 可以用 TodoWrite 工具管理任务列表：

```go
type TodoWriteParams struct {
    Title string    `json:"title"`
    Items []TodoItem `json:"items"`
}

type TodoItem struct {
    ID       string `json:"id"`
    Content  string `json:"content"`
    Status   string `json:"status"` // "pending" | "in_progress" | "completed"
    Priority string `json:"priority"` // "high" | "medium" | "low"
}
```

### 3.8 语言感知文本后处理

#### 参考
- `DeepVcodeClient/packages/core/src/utils/languageAwareTextProcessor.ts`

编辑后根据语言自动处理尾随空格、行尾格式等。

### 3.9 行尾检测与处理

#### 参考
- `DeepVcodeClient/packages/core/src/tools/line-endings.test.ts`

检测文件的换行符格式（LF/CRLF），编辑时保持一致。

---

## 4. 低优先级 —— 锦上添花

### 4.1 WebFetchTool（网页抓取）

> ⚠️ 从高优先级移入。对 coding agent 核心场景（读写代码）价值有限，属于锦上添花。

- **参考**: `DeepVcodeClient/packages/core/src/tools/web-fetch.ts`
- **价值**: 让 Agent 能抓取网页内容（查看文档、API 说明等）
- **关键特性**:
  - HTTP GET 获取 URL 内容
  - 响应大小限制
  - 内容格式化为 markdown

### 4.2 WebSearchTool（网络搜索）

> ⚠️ 从高优先级移入。同上，coding agent 核心不需要搜索引擎。

- **参考**: `DeepVcodeClient/packages/core/src/tools/web-search.ts`
- **价值**: 让 Agent 能搜索网络信息
- **关键特性**:
  - 调用搜索 API
  - 结果摘要返回
  - 域名过滤（允许/禁止）

### 4.3 BatchTool（批量工具调用）
- **参考**: `DeepVcodeClient/packages/core/src/tools/batch.ts`
- **价值**: 允许 Agent 在一次 tool call 中批量执行多个独立操作
- **场景**: 创建多个文件、读取多个文件
- **注意**: Agent 循环本身已支持并行 tool call，BatchTool 的主要价值在于将多次 tool call 合并为一次 LLM 交互以节省 token，实现复杂度较高

### 4.4 MultiEditTool（多文件编辑）
- **参考**: `DeepVcodeClient/packages/core/src/tools/multiedit.ts`
- **价值**: 一次 tool call 编辑多个文件

### 4.5 PatchTool
- **参考**: `DeepVcodeClient/packages/core/src/tools/patch.ts`
- **价值**: 基于 unified diff 格式的 patch 操作

### 4.6 MCP 工具集成
- **参考**: `DeepVcodeClient/packages/core/src/tools/mcp-client.ts` + `mcp-tool.ts`
- **价值**: 支持 MCP（Model Context Protocol）服务器发现和工具调用

### 4.7 沙箱检测
- **参考**: `DeepVcodeClient/packages/core/src/core/prompts.ts` 中的 `getDynamicSystemPrompt()`
- **价值**: 检测是否在沙箱/Docker/CI 环境中运行，调整系统提示（如禁用某些交互式功能）
- **场景**: 在 CI 环境中自动跳过确认流程，在容器中调整文件路径等

### 4.8 项目结构检测
- **参考**: `DeepVcodeClient/packages/core/src/utils/getFolderStructure.ts`
- **价值**: 启动时检测项目结构并注入到初始上下文

### 4.9 FullContext 模式
- **参考**: `DeepVcodeClient/packages/core/src/core/client.ts:554-601`
- **价值**: 启动时自动读取项目所有文件作为上下文

### 4.10 自定义模型支持
- **参考**: `DeepVcodeClient/packages/core/src/types/customModel.ts`
- **价值**: 允许用户配置自定义模型（OpenAI 兼容 / Anthropic 兼容）

### 4.11 速率限制和配额检测
- **参考**: `DeepVcodeClient/packages/core/src/utils/quotaErrorDetection.ts`
- **价值**: 检测 API 配额错误并自动切换模型

### 4.12 Session 管理器增强
- **参考**: `DeepVcodeClient/packages/core/src/services/sessionManager.ts`
- **价值**: 会话管理（保存、列出、恢复）

### 4.13 后台任务管理器
- **参考**: `DeepVcodeClient/packages/core/src/services/backgroundTaskManager.ts`
- **价值**: 管理 Shell 启动的后台进程，跟踪状态

### 4.14 文件操作队列
- **参考**: `DeepVcodeClient/packages/core/src/services/fileOperationQueue.ts`
- **价值**: 顺序化文件操作避免竞态

### 4.15 Telemetry / 遥测
- **参考**: `DeepVcodeClient/packages/core/src/telemetry/`
- **价值**: 工具调用计数、token 用量统计、性能指标

### 4.16 Hook 系统
- **参考**: `DeepVcodeClient/packages/core/src/hooks/types.ts`
- **价值**: 生命周期钩子（session start/end, before/after agent, pre-compress）

### 4.17 Policy Engine（策略引擎）
- **参考**: `DeepVcodeClient/packages/core/src/policy/policy-engine.ts`
- **价值**: 工具执行的策略规则引擎

### 4.18 MCP Response Guard
- **参考**: `DeepVcodeClient/packages/core/src/services/mcpResponseGuard.ts`
- **价值**: 保护 Agent 免受恶意 MCP 服务器响应攻击

---

## 5. 架构改进建议

### 5.1 Tool 接口增强

当前 pi-go 的 `Tool` 接口（`internal/agent/tool.go`）：

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any
    Validate(params json.RawMessage) (json.RawMessage, error)
    Execute(ctx context.Context, params json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error)
}
```

**已有的可选接口**（`internal/agent/tool.go`）：

```go
// ToolWithMode — 控制工具执行模式（并行/串行）
type ToolWithMode interface {
    Tool
    Mode() ExecutionMode
}

// ToolWithPromptInfo — 向系统提示注入工具使用指南
type ToolWithPromptInfo interface {
    Tool
    PromptInfo() PromptSnippet
}
```

**建议新增的可选接口**：

```go
// ToolWithConfirmation — 需要用户确认（§2.2 已详细描述）
// ToolWithKind — 声明工具类别（read/edit/execute/search），用于 MicroCompact 策略
// ToolWithLocations — 声明受影响的文件路径，用于编辑追踪和 PostCompactRestoration
// ToolWithAbortSignal — 支持取消执行，用于长时间运行的工具
// ToolWithBackground — 支持后台运行，用于 Shell 后台任务
// ToolWithSubAgent — 允许子 Agent 使用，用于 TaskTool 的工具子集选择
```

### 5.2 消息系统增强

DeepVcodeClient 的 `ToolResult` 有 `llmContent` 和 `returnDisplay` 分离：

```typescript
interface ToolResult {
    llmContent: PartListUnion;     // → 给 LLM 看的内容
    returnDisplay: string;         // → 给用户看的内容（markdown）
    visualDisplay?: VisualDisplay; // → 结构化渲染数据
}
```

pi-go 当前所有工具结果都合并成纯文本给 LLM 和用户。建议分离：

```go
type ToolResult struct {
    Content       string       // → 给 LLM 的原始内容
    DisplayText   string       // → 给用户展示的 markdown
    VisualDisplay *VisualData  // → 结构化展示数据（可选）
    IsError       bool
}
```

### 5.3 压缩系统分层

| 层级 | 触发条件 | 操作 | 成本 |
|------|----------|------|------|
| L0: MicroCompact | 空闲超时/token 70% | 清旧工具结果→占位符 | 零 LLM 调用 |
| L1: Full Compact | 压缩标记 + force | LLM 摘要历史 | 1 次 LLM 调用 |
| L2: PostCompactRestore | 压缩后 | 恢复最近文件内容 | 零 LLM 调用 |

### 5.4 系统提示构建器的可插拔设计

```go
type SystemPromptBuilder struct {
    // 用户可以注册多个 PromptSection，每个在最终 prompt 中贡献内容
    sections []PromptSection
}

type PromptSection interface {
    Name() string
    Content() string
    Priority() int // 排序优先级
}

// 内置 section
// 1. CoreBehavior   (最前面)
// 2. ToolSummary    (工具摘要)
// 3. ToolDetails    (工具详细描述)
// 4. Guidelines     (使用指南)
// 5. ProjectContext (CLAUDE.md)
// 6. Skills         (技能)
// 7. RuntimeInfo    (运行时信息，最末尾)
```

---

## 6. 按模块的详细对比表

### 6.1 工具集对比

| 工具 | pi-go | DeepVcodeClient | 优先级 |
|------|-------|----------------|--------|
| read_file | ✅ 基础实现 | ✅ 支持图片/PDF/Word/Excel | 🟡 中 |
| write_file | ✅ 基础实现 | ✅ 自动目录创建 | 🟡 中 |
| edit (replace) | ✅ 基础实现 | ✅ 编辑纠错/自动 lint/diff 展示 | 🔴 高 |
| bash (shell) | ✅ 默认禁用，需显式开启 | ✅ 危险命令检测/后台任务/Windows 编码 | 🔴 高 |
| grep | ✅ 基础实现 | ✅ 类似 | 🟢 低 |
| find | ✅ 支持 glob + 正则 | ✅ 独立 GlobTool，支持 .gitignore | 🟡 中（增强 .gitignore） |
| ls | ✅ 基础实现 | ✅ 类似 | 🟢 低 |
| read_many_files | ❌ 缺失 | ✅ glob 批量读取 | 🔴 高 |
| task (sub-agent) | ❌ 缺失 | ✅ 子 Agent 委派 | 🟡 中（实现复杂） |
| ask_user_question | ❌ 缺失 | ✅ 用户提问确认 | 🔴 高 |
| memory | ❌ 缺失 | ✅ 跨会话记忆 | 🟢 低（产品化特性） |
| edit_corrector | ❌ 缺失 | ✅ 模糊匹配建议 | 🔴 高（解决最高频失败） |
| delete_file | ❌ 缺失 | ✅ 安全删除 | 🟢 低（可用 Bash rm 替代） |
| todo_write | ❌ 缺失 | ✅ 任务管理 | 🟢 低（不扩展实际能力） |
| web_fetch | ❌ 缺失 | ✅ 网页抓取 | 🟢 低（非核心场景） |
| web_search | ❌ 缺失 | ✅ 网络搜索 | 🟢 低（非核心场景） |
| read_lints | ❌ 缺失 | ✅ 代码质量检查 | 🟡 中 |
| lint_fix | ❌ 缺失 | ✅ 自动修复 lint | 🟢 低 |
| lsp tools (6个) | ❌ 缺失 | ✅ LSP 代码智能 | 🟡 中 |
| batch | ❌ 缺失 | ✅ 批量工具调用 | 🟢 低 |
| multiedit | ❌ 缺失 | ✅ 多文件编辑 | 🟢 低 |
| patch | ❌ 缺失 | ✅ unified diff patch | 🟢 低 |
| mcp tools | ❌ 缺失 | ✅ MCP 集成 | 🟢 低 |
| ppt tools | ❌ 缺失 | ✅ PPT 工具 | 🟢 低 |

### 6.2 服务层对比

| 服务 | pi-go | DeepVcodeClient | 优先级 |
|------|-------|----------------|--------|
| 上下文压缩 | ✅ 基础 LLM 摘要 | ✅ 三级策略 + 动态模型升级 | 🔴 高 |
| 循环检测 | ❌ 缺失（仅有 maxTurns 限制） | ✅ 多策略检测 | 🔴 高 |
| 危险命令检测 | ❌ 缺失 | ✅ 黑名单/白名单 | 🔴 高 |
| 确认系统 | ❌ 缺失 | ✅ 5 种确认类型 + 6 种确认结果 | 🔴 高 |
| 编辑纠错 | ❌ 缺失 | ✅ 模糊匹配建议 | 🔴 高（解决最高频失败） |
| 文件操作队列 | ❌ 缺失 | ✅ 竞态保护 | 🟢 低 |
| 后台任务管理 | ❌ 缺失 | ✅ 进程追踪 | 🟡 中 |
| Telemetry | ❌ 缺失 | ✅ 监控/指标 | 🟢 低 |
| Policy Engine | ❌ 缺失 | ✅ 策略规则 | 🟢 低 |
| Hook 系统 | ❌ 缺失 | ✅ 生命周期钩子 | 🟢 低 |

### 6.3 AI 层对比

| 特性 | pi-go | DeepVcodeClient |
|------|-------|-----------------|
| Provider 数量 | 3 (anthropic/openai/mock) | 1 (DeepV Server, 但协议层支持 Anthropic/Gemini) |
| 流式事件 | TextDelta/ToolCall/Error/Done | TextDelta/ToolCall/TokenUsage/Error/LoopDetected |
| 重试机制 | ✅ 基础重试 | ✅ 指数退避重试 |
| Token 统计 | ✅ 基础 | ✅ 详细 |
| 模型切换 | ❌ 缺失 | ✅ 自动降级 |
| 系统提示缓存边界 | ❌ 缺失 | ✅ 静态/动态分离 |
| 多 Agent 风格 | ❌ 缺失 | ✅ 6 种风格 |

---

## 7. 实施路线图建议

### 第一阶段（基础设施加固）—— 5-7 天

1. **Tool 接口增强**
   - 在已有 `ToolWithMode` / `ToolWithPromptInfo` 基础上，新增 `ToolWithConfirmation` 可选接口
   - 新增 `ToolWithKind`（工具类别标记，用于 MicroCompact 策略）
   - 新增 `ToolResult.DisplayText` 分离（LLM 内容 vs 用户展示内容）

2. **Agent 循环增加确认机制**
   - 添加 `EventAwaitingConfirmation` 事件
   - 添加 `Confirm`/`Reject` 方法（基于 channel + 超时）
   - 交互模式：TUI 渲染确认提示
   - Serve 模式：SSE 新增 `awaiting_confirmation` 事件 + `/chat/:id/confirm` 端点
   - Print 模式：默认自动确认

3. **循环检测**
   - 实现 `LoopDetector`（相同参数/相同工具名/重复文本/LLM 辅助四种策略）
   - 集成到 agent loop 中，检测到循环时 emit 事件并停止

4. **编辑纠错（Edit Corrector）**
   - 当 EditTool 的 `old_string` 匹配失败时，自动计算最近距离建议
   - 将建议附加到错误信息中，帮助 LLM 在下一轮自动修正

### 第二阶段（工具补充）—— 5-7 天

1. **高优先级工具实现**
   - `ReadManyFilesTool`（批量读文件 + glob 匹配 + 字符预算）
   - `AskUserQuestionTool`（Agent 向用户提问确认）
   - 增强 `FindTool`（增加 .gitignore 支持）

2. **Shell 工具安全增强**
   - 危险命令检测（黑名单机制）
   - 后台任务支持

3. **Edit 工具增强**
   - Diff 上下文展示
   - 自动 Lint 检查（编辑后）

### 第三阶段（上下文管理优化）—— 3-5 天

1. **MicroCompact 服务**（零 LLM 调用，清旧工具结果→占位符）
2. **PostCompactRestoration 服务**（压缩后自动恢复最近读取的文件）
3. **多级压缩策略整合**（L0: MicroCompact → L1: FullCompact → L2: PostCompactRestore）

### 第四阶段（IDE 智能）—— 5-7 天

1. **LSP 集成基础框架**
2. **LSP Hover / GoToDefinition / FindReferences**
3. **ReadLints / LintFix 工具**

### 第五阶段（体验优化）—— 可选

1. 多 Agent 风格系统
2. Batch / MultiEdit 工具
3. MCP 集成
4. Telemetry
5. Web 工具（WebFetch / WebSearch）
6. Memory 工具（跨会话记忆）

---

## 总结

pi-go 作为 pi 的 Go 重写，已经搭建了坚实的 Agent 框架底座。DeepVcodeClient 作为商业化产品，在以下三个维度提供了大量可吸收的设计思路：

1. **安全性**：确认机制（5 种确认类型）、危险命令检测、循环检测（4 种策略）—— Agent 不会"失控"
2. **工具生态**：30+ 工具覆盖读取/搜索/编辑/执行/代码智能全链路 —— Agent 能做的事更多
3. **上下文智能**：三级压缩（MicroCompact → FullCompact → PostCompactRestore）—— Agent 能处理更长的会话

### 建议实施顺序

| 阶段 | 内容 | 预计时间 | 收益 |
|------|------|----------|------|
| **P0（紧急）** | 循环检测 + 确认机制 | 3-5 天 | 防止 Agent 失控、保护 API 配额 |
| **P1（高）** | Edit Corrector + AskUserQuestion + ReadManyFiles | 5-7 天 | 解决最高频失败 + 扩展 Agent 交互能力 |
| **P2（中）** | MicroCompact + 危险命令检测 + FindTool 增强 | 3-5 天 | 长会话稳定性 + 安全加固 |
| **P3（中）** | TaskTool/SubAgent + PostCompactRestoration | 5-7 天 | 复杂任务分解 + 压缩后上下文保持 |
| **P4（低）** | LSP / Web / MCP / Telemetry 等 | 按需 | 锦上添花 |

**优先实现 P0 + P1**，这些是打造安全可靠 Agent 的基石，且能立即提升 Agent 的实用性和可靠性。
