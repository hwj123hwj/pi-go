> 历史文档：本文保留更名前的 pi-go 项目记录；项目现名为 EasyAgent。

# 决策：pi-go 收敛为企业开箱即用 Agent——原生管理生态路线

日期：2026-09-25
状态：提议（待实施）
关联：[server-admin-plane-roadmap.md](server-admin-plane-roadmap.md)（管理面 API 细分）、[pg-storage.md](pg-storage.md)（存储选型）

## 背景

pi-go 从"个人 Agent 底座 + 三形态交付"收敛为**企业开箱即用 Agent**：自托管、单二进制、飞书原生，IT 一条命令部署进内网，员工在飞书里用，管理员有控制台。该定位下没有好的开源占位者——Dify/Coze 类工作流平台走的是画布编排路线，与 pi-go 的运行时路线不同。

参照系：本地 Desktop/mevo 生态（**只读架构参考，公司/客户项目，不搬代码、不引依赖、不外泄任何配置**）。mevo 围绕 openclaw 建管理生态：openclaw 是外部黑盒实例，因此需要 botmanager（实例生命周期：套餐/镜像/升级/冻结）、meter（从对象存储收会话归档做计量）、MevoSafe（旁路内容安全）+ admin 控制台，四套系统围着外星进程转。

## 决策

### 1. 原生模式是结构性优势

pi-go 的 agent 是进程内 session，管理面可以是内核一等公民：mevo 需要 botmanager + meter + safe 三个旁路系统才能获得的能力，在 pi-go 里分别是"一张表 + 一个 handler"：

| mevo 旁路系统 | 职责 | pi-go 原生对应 |
|---|---|---|
| botmanager | 实例 provision/升级/冻结/镜像 | 不需要——无实例舰队，session 即 agent；冻结 = 禁用 user |
| meter | 会话归档 → 人工用量计量 | `usage` 表 + `cost.go`，实时落库，无归档管道 |
| MevoSafe | 旁路敏感信息过滤 + 熔断 | `sdk/policy` 规则 + audit log，请求路径内原生执行 |
| admin 控制台 | 串起以上三者的 Next.js 应用 | P2 内嵌控制台页签，复用同一二进制 |

### 2. 管理面信息架构 v1（从 mevo-agent-admin 提炼裁剪）

保留五组：**总览**（仪表盘、Token 监控、财务口径的用量报表）；**用户与会话**（用户管理、会话管理、禁用/解禁）；**模型与技能**（LLM 模型管理、网关同步、技能/MCP 开关）；**安全与审计**（policy 规则、敏感操作审计、异常扫描 v0）；**运维**（操作日志、健康、版本信息）。

明确不做（mevo 有、pi-go 不做）：实例舰队管理（套餐/镜像/灰度升级——单二进制无此问题）；商业化模块（商品/订单/激活码——内部企业工具非 SaaS，将来做对外运营再立项）；站内信（飞书本身就是消息面）。

### 3. 产品形态收敛（修订 PRODUCT_ROADMAP 三形态）

- **主形态：Server + 飞书**（企业入口）。
- CLI 保留，降级为开发/运维工具。
- Desktop 搁置，其"Agent 工作台"职责由 P2 Web 控制台接管，不维护两套 UI。
- personal 特性（音乐、个人画像）不删代码，收进 `personal` 模式开关或独立 profile，企业部署默认关。

### 4. 企业基座路线（在 server-admin-plane-roadmap 上叠身份层）

- **P0 安全收紧**（不变，前置）：文件端点限制在会话 workspace 内；serve 模式强制 API key（留 dev 旁路）；CORS 收敛。
- **P0.5 身份模型最小版**（新增）：principal 概念——飞书 `senderOpenID` 与 HTTP API key 都解析为 User；session 记 owner；config 拆 global + per-user 覆盖（模型选择先行，复用 `switchModel`）；RBAC v0 仅 admin/member 两角色。
- **P1 管理面 API + 企业件**：原定 `/policy` `/config` `/stats`、工具注册持久化之外，新增 audit log（工具调用/文件访问/会话操作，结构化 JSONL→PG）、per-user 限流与 token 预算、`/stats` 按用户聚合。
- **P2 控制台 + 交付件**：内嵌控制台五组页面；**Dockerfile + docker-compose（app + PG）**——"开箱即用"的箱就是这个 compose 文件。
- 观察项：SSO（飞书免登已有基础）、多实例部署（先不做，单二进制 + PG 已覆盖目标规模）。

### 5. 此前两项评估在新框架下的位置

- **workflow**（ZCode 移植评估）：从个人编排工具升格为**团队自动化**——YAML 管道 + 定时触发 + 飞书里触发，进 P1 后实现；Go 原生重设计结论不变（不搬 TS 引擎）。
- **computer use**（ZCode 闭源确认）：服务器无 GUI，企业场景优先级放低；维持 MCP 接入 + policy 管权限。

## 备选与放弃原因

- **围绕 openclaw 复制 mevo 模式**（引入 openclaw 当 agent 引擎、旁路建管理生态）：pi-go 已拥有等价且更深的运行时，旁路模式徒增四套系统；且 openclaw 生态与飞书集成深度不如现有 feishu bridge。
- **三形态齐头并进**：单人维护力量分散，Desktop 与 Web 控制台职责重叠。
- **直接抄 mevo-agent-admin 的技术栈**（Next.js + FastAPI + MySQL + Redis）：pi-go 是 Go 单二进制 + PG，栈不一致，只取信息架构。

## 影响

- `docs/PRODUCT_ROADMAP.md` 的三形态定位需按本文档修订（另起文档更新，不在本决策内直接改）。
- personal 模式开关需要 config 层支持，进 P0.5。
- 管理面 API 清单以 server-admin-plane-roadmap.md 为准，本文档不重复定义。
- mevo 目录仅作只读架构参考，任何设计引用指向本文档，不指向 mevo 内部文件。
