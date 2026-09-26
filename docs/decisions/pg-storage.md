> 历史文档：本文保留更名前的 pi-go 项目记录；项目现名为 EasyAgent。

# 决策：企业形态存储选型——PostgreSQL 关系面 + 文件面分层

日期：2026-09-25
状态：提议（待实施）
关联：[enterprise-convergence.md](enterprise-convergence.md)（企业收敛总路线）

## 背景

企业收敛后出现四类新的持久化需求：身份/RBAC、审计、配额/用量账本、管理面聚合查询。pi-go 现状是纯文件存储（`sdk/session` JSONL + `sdk/sessionmgr` JSON 元数据 + `internal/kbvector` 自研 JSON 向量存储），单写者模型（单 store 单 mutex + fsync）。此前管理面路线中曾设想"JSONL 先行、SQLite 过渡"，随企业定位明确一并修正。

## 决策

### 1. 关系型数据面用 PostgreSQL

进 PG 的数据：users/roles（RBAC v0）、session 索引（id/owner/model/状态/时间）、audit log（工具调用/文件访问/会话操作）、usage/quota 账本、policy 规则、workflow 定义与运行记录。

理由：

1. **并发模型**：上述数据全是多租户并发写；JSONL 单写者模型和 SQLite 单写者都别扭，PG 原生匹配。
2. **查询面**：`/stats` 按人/按天/按工具聚合是 SQL 一句话；文件方案等于自研扫描器。
3. **运维生态即企业级**：企业 IT 本来就运维 PG（pg_dump/WAL 归档/监控/主备是现成技能），让其接管自定义存储才是部署摩擦。litellm-gateway 已跑在 PG 上，部署栈中已存在。
4. **pgvector**：`internal/kbvector` 自研 JSON 向量存储换 pgvector 后获得 HNSW/IVFFlat 索引，少维护一个自研组件（排 P2，随控制台知识库页一起）。

### 2. 分层边界——不是什么都进 PG

- **进 PG**：上节所列关系面。
- **暂留文件**：会话消息正文（transcript JSONL）。append-only、按 session 单写者、fsync 已做对，迁移收益低；等出现"历史会话服务端全文检索"或"多实例共享存储"需求再进 PG jsonb。副作用是 PG 库小、备份快。
- **保持文件**：`mcp.json`、`.pi-go/` 下的工具注册等配置类数据，归配置面管。

### 3. "开箱即用"与外部依赖的调和

`PI_GO_DB_URL` 单一入口：企业模式无 DSN 直接拒绝启动（fail fast）；dev/personal 模式无 DSN 走文件继续可用；docker-compose 把 app + PG 打成一个箱（"开箱即用"的箱就是 compose 文件），企业自带 PG 实例则给 DSN 即可。

### 4. 技术选型

- 驱动：pgx（Go 事实标准、纯 Go 无 cgo）。
- 迁移：goose 或 golang-migrate，schema 随版本进仓库。
- **不做 SQLite/PG 双方言抽象**：维护两套方言的成本远高于省一个依赖；将来若真需要嵌入式模式，用 modernc/sqlite 单实现评估，不进本决策承诺。

## 备选与放弃原因

- **SQLite 过渡**（原管理面路线设想）：单写者不适合多租户并发账本；企业 IT 陌生，备份/监控/主备生态弱于 PG；既然终态是企业 PG，过渡步是弯路。
- **全部进 PG（含 transcript）**：首期收益低（无检索需求），库体膨胀拖慢备份。
- **mevo 式旁路 meter**（会话归档 → 对象存储 → 离线管道计量）：非实时的、为外部黑盒 agent 发明的管道；pi-go 原生实时落库即可。

## 影响

- P0.5 身份模型直接建在 PG 上（先 PG 后身份，顺序不可反）。
- `deploy/` 新增 Dockerfile 与 docker-compose.yml；`docs/CONFIG.md` 增加 `PI_GO_DB_URL` 与企业/个人模式说明。
- kbvector 接口保留，P2 换 pgvector 实现。
- PG 引入后 `go.mod` 新增 pgx/goose 依赖；dev 流程新增"compose 起本地 PG"说明。
