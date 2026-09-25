# 决策：服务端管理面（看板/配置）路线——参照上游 pi 的对照与建设顺序

日期：2026-09-25
状态：提议（待实施）

## 背景

pi-go 的服务端形态定位为"运行在服务器端、通过飞书使用的 Agent 服务"（PRODUCT_ROADMAP.md 三形态之一）。用户提出：安装启动后是否需要一个看板，能配置系统化内容——动态工具、权限等。为回答该问题，克隆上游 `earendil-works/pi`（TypeScript monorepo，工作区 `../pi`，仅作参考、不进 PROJECTS.md）做了逐项对照。

## 对照：上游 pi vs pi-go 现状

| 维度 | 上游 pi | pi-go 现状 | 判断 |
|---|---|---|---|
| 对外协议 | 分层协议栈：`pi-protocol`（CBOR 分帧、版本握手、attachmentId 路由围栏）+ `pi-client`（传输无关）+ `pi-server`（透明路由，不解码业务数据） | REST + SSE + 自定义 WebSocket（prompt/cancel/switch_model/ping） | 不重写协议。REST/SSE 对 Web 与飞书更顺手，二进制协议是上游为多 transport 付的成本 |
| 会话模型 | 持久 Session 支持多 presentation attach：多端挂同一会话，路由围栏、断线只释放自己的 attachment；会话是事件树（分支） | sessionmgr JSON 元数据 + `sdk/session` JSONL 消息存储；一个会话一个消费方，无 attach 语义 | 多端 attach 是三端（飞书/Web/桌面）共用会话的正确语义，列为观察项 |
| 持久化 | `pi-durable`：Memory/JSONL/SQLite 可插拔 | JSONL + JSON 文件 | 够用，暂不动 |
| 动态工具 | 进程内 TS 扩展（registerTool/命令/事件/Provider）；RPC 模式（JSONL 子进程协议）供 IDE/外部 UI 驱动；无 MCP 一等支持（docs 全仓无 MCP） | `POST /tools/register`（内存、重启丢失）+ `internal/mcp`（`.pi-go/mcp.json`，stdio/SSE/HTTP） | pi-go 的 MCP 一等支持领先上游；缺注册持久化与启停/删除端点 |
| 权限/隔离 | 明确不做内置权限系统；`containerization.md` 给三模式（Gondolin micro-VM 扩展 / 整进程 Docker / OpenShell） | `sdk/policy` 规则引擎（模式匹配、优先级、会话缓存、`policy.json` 持久化）+ Updater | 两边各选一头：上游选隔离边界，pi-go 选策略引擎。长期两层都要——policy 管体验层确认，隔离交部署层 |
| 配置面 | settings.md 全量 settings 引用，项目/agent 目录分层覆盖 | `PI_GO_*` 环境变量 + 配置文件，改配置需重启；模型可按会话切换、可从网关动态拉取（PR #40） | 缺运行时查看/修改的安全子集 |
| 监控/看板 | 无任何 dashboard（全仓 grep 证实）；TUI 为主 + RPC 给第三方 UI + 独立 pi-chat 仓库做聊天自动化 | 嵌入式 Web UI（聊天界面）+ REST 会话/消息/diff；`sdk/ai/cost.go` 有 token 成本；无聚合统计 | 看板属于超出上游的自主创新 |

## 决策

1. **看板定位为"管理面"，先 API 后 UI**。上游没有这个东西，不照抄；它的价值在于管理面 API 同时服务飞书 bridge、桌面端和 Web，符合"能力沉淀在共用层"原则。
2. **建设顺序**：
   - **P0 安全收紧（前置条件）**：`GET/PUT /sessions/{id}/file`、`/workspace/read-file`、`/workspace/write-file`、`/workspace/list-dir`、`/workspace/search-files` 目前接受任意绝对路径，可读写服务器任意文件（server.go:964/1082/1140/1208/1267）；CORS 为 `*`，API key 可选。改为限制在会话 workspace 内、serve 模式强制 API key、CORS 收敛到配置白名单。
   - **P1 管理 API**：`GET/PUT /policy`（读写规则 + 热加载，数据源 `sdk/policy` Updater）；`GET/PATCH /config`（安全子集：provider/model/网关/开关）；工具注册持久化到 `.pi-go/external-tools.json` + `DELETE /tools/{name}` + 启停开关；`GET /stats`（token 用量、成本、会话数、工具调用频次，数据源 cost.go 与运行日志）。
   - **P2 控制台 UI**：在 `internal/web/static` 现有嵌入 UI 加"控制台"页签（会话列表、模型、工具开关、policy 规则、用量图表），不新起前端项目。
   - **观察项（不立项）**：多 presentation attach。等飞书/Web/桌面真实需要共用同一会话时，参照上游 pi-server 的 attachment/围栏语义再设计。
3. **明确不做**：CBOR/二进制协议重写；进程内脚本扩展机制（pi-go 用 MCP + HTTP 外部工具对齐自身 Go 架构）；内置沙箱（沿用"沙箱列入永不规划"决策 d1d9399，隔离交给部署层：容器/网关/k8s）。

## 备选与放弃原因

- **引入上游 pi-server/chord 的 Go 等价物**：透明路由 + 服务组合是为多 transport 多租户付的复杂度，单人项目过度设计；REST/SSE/WS 已覆盖现有三端。
- **看板先做 UI 后做 API**：UI 会把 API 形状焊死，飞书侧与桌面端无法复用，违背共用层原则。
- **把 policy 引擎升级为内置沙箱**：与"沙箱永不规划"决策冲突，且隔离在部署层做更彻底（上游 containerization 三模式皆此思路）。

## 影响

- 新增端点：`/policy`、`/config`、`/stats`、工具注册持久化；`docs/API.md`、`docs/CONFIG.md` 需同步更新。
- `deploy/` 的默认配置在 P0 后需重新评估（强制 API key 会改变本地开发体验，需提供 dev 旁路）。
- 上游参考仓库 `../pi`（earendil-works/pi）仅作设计参照，不参与构建，不进 PROJECTS.md；后续对齐决策引用本文档而非重新克隆分析。
