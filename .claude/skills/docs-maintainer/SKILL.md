---
name: docs-maintainer
description: "维护 pi-go 项目文档结构与一致性。当用户提到'维护一下文档'、'更新文档'、'文档整理一下'、'docs 该整理了'、'同步文档和代码'、'整理 docs 目录'等意图时加载此 skill。"
---

# docs-maintainer

## pi-go 项目路径

**绝对路径**：`/Users/weijian/Desktop/develop/test/pi/pi-go`

所有路径基于此根目录，下文简称 `{pi-go}`。

## 文档结构规范

本 skill 维护以下目录结构，迁移时必须遵守：

```
docs/
├── README.md                          # 文档总索引
├── PROJECT_CONTEXT.md                 # 项目上下文快照
├── PRODUCT_ROADMAP.md                 # 产品路线图（长期）
├── CONTRIBUTING.md                    # 贡献指南（长期）
├── deploy.md                          # 部署说明（长期）
│
├── references/                        # 稳定查阅资料（接口/集成/项目快照）
│
├── decisions/                         # 当前采纳判断（会演进，但比 research 更接近团队共识）
│
├── research/                          # 外部项目调研原始报告（天然会过期）
│
├── dev/                               # 开发文档（4-agent 流水线产出）
│   └── {topic}/                       # 一个主题一个目录
│       ├── proposal.md                # 计划 agent 产出
│       ├── review.md                  # 审核 agent 产出
│       └── execution-plan.md          # 确认后的执行文档
│
└── archive/                           # 已完成的开发主题
    └── {topic}/                       # 整个目录从 dev/ 移入
```

### dev/ 文档元信息头

每个 `dev/` 下的文档必须有 YAML 头：

```yaml
---
status: draft | reviewed | approved | done
author: plan-agent | review-agent | exec-agent | research-agent
created: YYYY-MM-DD
updated: YYYY-MM-DD
reviewer: review-agent          # review.md 专有
review-status: pending | approved | rejected | needs-revision  # review.md 专有
depends-on: []                  # 可选
---
```

状态流转：`draft → reviewed → approved → done`（rejected/needs-revision 回到 draft）

### 文档归属判断

| 性质 | 归属 | 示例 |
|------|------|------|
| 长期有效的项目级文档 | `docs/` 根目录 | PROJECT_CONTEXT、ROADMAP、CONTRIBUTING |
| 稳定查阅资料 | `docs/references/` | 第三方集成参考、项目快照、接口说明 |
| 基于调研得出的当前采纳判断 | `docs/decisions/` | skills vs application、goal/compact 取舍 |
| 外部项目调研原始分析 | `docs/research/` | 竞品分析、框架对比、源码调查 |
| 4-agent 流水线产出的开发文档 | `docs/dev/{topic}/` | 提案、审核、执行计划 |
| 已完成的开发主题 | `docs/archive/{topic}/` | 已归档的执行计划 |

---

## 维护流程

### 阶段一：扫描现状（并行执行）

#### 1. 扫描源码结构

```bash
find {pi-go}/internal -type f -name "*.go" | head -80
grep -r "^type.*interface {" {pi-go}/internal --include="*.go" -l
grep -r "^func New" {pi-go}/internal --include="*.go" | head -30
```

#### 2. 动态扫描所有文档

**不要写死文件列表**，用 `find` 动态发现：

```bash
find {pi-go}/docs -name "*.md" -type f | sort
```

对每个文件：
- 读取内容（至少前 50 行 + 关键章节），判断文档类型和状态
- `dev/` 下的文档：检查元信息头是否完整、状态是否合理
- `docs/README.md`：检查索引是否覆盖所有文件
- `docs/PROJECT_CONTEXT.md`：检查与源码一致性
- `archive/` 下的文件：只读头部，判断归档是否合理

### 阶段二：差异分析

对每份文档做以下检查：

| 检查项 | 判断标准 |
|--------|---------|
| **索引缺失** | `docs/README.md` 没有列出存在的文件 |
| **索引多余** | `docs/README.md` 列出了但文件已不存在 |
| **内容过时** | 文档描述与当前源码不一致 |
| **归档判断** | `dev/` 下 `status: done` 的主题 → 应归档到 `archive/` |
| **元信息头** | `dev/` 下的文档缺少 YAML 头或字段不完整 |
| **归属错误** | 文档放在错误的目录（如参考资料放在根目录） |
| **决策/调研混放** | 带明确采纳建议与路线图的文档放在 `research/` 或 `references/` |
| **PROJECT_CONTEXT** | 架构、能力表、文件索引与实际代码不匹配 |
| **learning 索引** | `learning/README.md` 中引用的 docs/ 路径已失效 |

### 阶段三：执行修正

#### docs/README.md 更新

- 补全缺失的索引条目（按目录分组：长期文档 / 参考资料 / 决策文档 / 调研报告 / 开发文档 / 归档）
- 移除已不存在文件的条目
- 更新 `dev/` 主题的状态列

#### PROJECT_CONTEXT.md 更新

- **架构图**：`internal/` 目录结构变化时更新
- **核心能力表**：新增/删除的能力
- **关键接口表**：接口签名变化
- **关键文件速查**：文件存在性和职责准确性
- **状态标记**：✅ 已完成 / 🔲 规划中 / 🔄 进行中

#### 归档操作

满足以下条件时，将 `dev/{topic}/` 整个目录移入 `archive/`：
- 主题下所有文档的 `status` 均为 `done`
- 或用户明确指示归档

```bash
mv {pi-go}/docs/dev/{topic} {pi-go}/docs/archive/{topic}
```

**不要归档**：PROJECT_CONTEXT、ROADMAP、CONTRIBUTING、deploy.md

#### 归属修正

文档放错目录时，移到正确位置：
- 稳定查阅资料 → `references/`
- 决策文档（结合调研与当前代码给出采纳结论）→ `decisions/`
- 调研报告（分析外部项目的原始调查）→ `research/`
- 开发文档（4-agent 产出）→ `dev/{topic}/`

#### 元信息头补全

`dev/` 下缺少 YAML 头的文档，根据内容推断并补全。

#### learning/README.md 路径校验

`learning/` 属于 pi（TypeScript 版）项目，不属于 pi-go docs 维护范围。但其中引用了 `docs/research/` 和 `docs/dev/` 的路径，维护时只检查这些路径是否仍然有效，失效则列入"需人工确认"，不主动修改 `learning/README.md` 的内容。

### 阶段四：输出维护报告

```
## 文档维护报告 {YYYY-MM-DD}

### 已修正
- [更新] docs/README.md：补充了 X 的索引
- [归档] dev/zzz/ → archive/zzz/
- [更新] PROJECT_CONTEXT.md：核心能力表新增 W
- [归属] feishu-ref.md → references/

### 无需修正
- CONTRIBUTING.md：内容准确
- deploy.md：内容准确

### 需人工确认
- XXX.md：描述了尚未确定的设计方向，建议确认后更新
```

---

## 写作标准

1. **只改该改的**：不要重写内容准确的文档
2. **保持风格一致**：沿用现有格式，不引入新模板
3. **谨慎归档**：宁可多保留也不误归档，有疑问的放进"需人工确认"
4. **不要删文件**：只做移动（归档）和内容更新
5. **不改代码**：只维护文档，发现代码问题列到"需人工确认"
