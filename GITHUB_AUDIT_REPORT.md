# GitHub 仓库全面审计报告

> 用户: `hwj123hwj` (WJ Huang / 黄威健)
> 审计时间: 2026-06-01
> 仓库总数: 24 (含 3 个 fork)

---

## 一、仓库全景

### 1.1 仓库分类总览

| 分类 | 仓库 | 数量 |
|------|------|------|
| **核心框架项目** | pi-go, easyagent, litellm-gateway | 3 |
| **Agent 记忆/人格系统** | jarvis-memory, friday-memory, dabai-memory, lobster-legion, hermes-vault | 5 |
| **技能/工具生态** | custom-skills, agent-lessons, agent-secrets, growth-agent | 4 |
| **微信生态** | wechat-api, wechat-radar, wx_key (fork) | 3 |
| **学术/学习** | VibeCheck, go-learning-for-java-dev, knowledge-base-rd, pku-exam | 4 |
| **Fork 研究用** | openhanako (fork), wechat-radar (fork), wx_key (fork) | 3 |
| **其他/废弃** | career-profile, vid-publisher, side-hustle-ideas, multimodal-transport | 4 |

### 1.2 活跃度分布

| 状态 | 仓库 | 说明 |
|------|------|------|
| **活跃 (5月仍有提交)** | jarvis-memory, custom-skills, agent-lessons, litellm-gateway, pi-go, VibeCheck | 6 个 |
| **近期暂停 (4月中-5月初)** | openhanako, easyagent, hermes-vault, friday-memory, lobster-legion | 5 个 |
| **停滞 (>30天无更新)** | wechat-api, vid-publisher, dabai-memory, go-learning-for-java-dev 等 | 13 个 |

### 1.3 AI 辅助开发占比

| 仓库 | 总提交 | Claude 辅助 | 占比 |
|------|--------|-------------|------|
| pi-go | 61 | 48 | **79%** |
| easyagent | 100+ | 30 | 30% |
| litellm-gateway | 43 | 11 | 26% |
| custom-skills | 62 | 26 | 42% |

**pi-go 的 79% AI 辅助率非常高**，说明你深度使用 Claude Code 进行开发。

---

## 二、核心问题诊断（狠狠找的问题）

### 🔴 问题 1：严重的安全隐患——Token/密钥泄露

**涉及仓库**: jarvis-memory, lobster-legion, agent-secrets, 多个 memory 仓库

| 泄露内容 | 位置 | 严重度 |
|----------|------|--------|
| GitHub PAT `ghp_TGM1z...` | jarvis-memory/DREAMS.md, 今天对话中 | 🔴 极严重 |
| SSH 服务器 IP/端口/用户名 | jarvis-memory/TOOLS.md | 🔴 严重 |
| 飞书 Session Key | jarvis-memory/memory/ 日志 | 🟡 中等 |
| GitLab Token | lobster-legion (已在 4-23 清理，但 git history 仍存在) | 🟡 中等 |
| 小红书 Cookies | agent-secrets 仓库 | 🟡 中等 |

**问题本质**：虽然你在 4-23 做了一轮安全清理（lobster-legion 中移除明文密钥），但：
1. **Git history 永久保留**——即使文件删除，`git log -p` 仍可看到
2. 清理不彻底——jarvis-memory 中仍有泄露
3. `agent-secrets` 仓库本身就是用来存密钥的，但它是 private repo——问题是 token 出现在了 DREAMS.md 的日志中

**改进方案**：
- 立即撤销你刚才对话中暴露的 `ghp_TGM1z...` token
- 对所有 memory 仓库做 `git filter-branch` 或 BFG 清理 git history
- 将敏感信息统一到 `.env` 或 secrets manager，日志中绝不打印

---

### 🔴 问题 2：项目碎片化——24 个仓库，13 个停滞

**数据**：
- 总仓库数 24，但 **13 个 (>50%) 超过 30 天无更新**
- 仅 1 次提交的仓库有 4 个：`career-profile`、`growth-agent`、`pku-exam`、`vid-publisher`
- 活跃开发的仅 6 个

**问题本质**：你不断启动新项目，但很难持续跟进。这导致：

```
2026-01  wx_key (fork)
2026-03  知识管理 → 龙虾军团 → 副业计划 → Go学习 → 微信 → 多式联运
2026-04  growth-agent → agent-secrets → litellm → hermes → vid-publisher
2026-05  custom-skills → easyagent → openhanako → pi-go → wechat-radar → pku-exam → career-profile
```

**每个月都在切换方向**。3 月做 Agent 记忆系统，4 月做 LLM 网关，5 月同时做 pi-go + easyagent + custom-skills + 微信雷达 + 考研计划...

**改进方案**：
- **砍到最多 3 个活跃项目**，其他的 archive 或 delete
- 建议保留：pi-go（核心技术栈）、custom-skills（有 star 有生态）、1 个研究性项目
- 其余设为 Archived，避免分散注意力

---

### 🔴 问题 3：核心项目之间定位重叠

你有 **3 个 Agent 框架项目**，定位互相冲突：

| 项目 | 语言 | 定位 | 状态 |
|------|------|------|------|
| **pi-go** | Go | 纯服务端 Agent 框架 | 活跃 |
| **easyagent** | Python | 飞书原生 Agent 框架 | 5-21 后暂停 |
| **openhanako** | JS/TS | 桌面端 Agent 应用（fork） | 纯 fork，无贡献 |

加上 pi 原版（TypeScript monorepo，你用来学习的项目），你同时在关注 **4 个 Agent 框架**。

**问题本质**：
- pi-go 和 easyagent 都在做 Agent Loop + Tool 系统 + 会话持久化，只是语言不同
- 但两者的设计理念差异大（Go 四层分层 vs Python 从 Hermes 移植），没有共享设计
- openhanako 是纯 fork，没有贡献回上游，也没有自定义改动

**改进方案**：
- **选定一个主力 Agent 框架**，深度投入
- 如果是 pi-go（Go 是你的学习方向），就暂停 easyagent
- openhanako 如果只是为了学习，archive 它
- 从 easyagent 的飞书深度集成中学到的经验，迁移到 pi-go

---

### 🟡 问题 4：开源运营缺失——0 Star / 0 Issue / 0 PR

**数据**（排除 fork 和 memory 仓库）：

| 仓库 | Stars | Forks | Issues | License | Topics |
|------|-------|-------|--------|---------|--------|
| pi-go | 0 | 0 | 0 | ❌ 无 | ❌ 无 |
| easyagent | 0 | 0 | 0 | ❌ 无 | ❌ 无 |
| litellm-gateway | 0 | 0 | 0 | ❌ 无 | ❌ 无 |
| custom-skills | 3 (1 自己) | 0 | 0 | ❌ 无 | ❌ 无 |
| VibeCheck | 0 | 0 | 0 | ❌ 无 | ❌ 无 |
| wechat-api | 0 | 0 | 0 | ❌ 无 | ❌ 无 |
| wechat-radar | 0 | 0 | 0 | ❌ 无 | ❌ 无 |

**你的 21 个原创仓库（不含 fork）总共只有 3 个 star**（其中 1 个还是你自己的）。

**问题本质**：
- **没有 License** = 法律上别人不能用你的代码
- **没有 Topics** = GitHub 搜索发现不了
- **没有 Issues** = 没有反馈渠道，纯单向输出
- **没有 README 截图/Logo** = 第一印象差

**改进方案**：
1. 给 pi-go 和 custom-skills 添加 MIT 或 Apache 2.0 License
2. 给所有项目添加 GitHub Topics（`agent`, `go`, `llm`, `ai-agent` 等）
3. pi-go 写英文 README + 架构图 + Quick Start
4. 在相关社区（V2EX、Reddit r/LocalLLaMA、GitHub Discussions）分享

---

### 🟡 问题 5：技术深度 vs 广度的平衡

从仓库分布看你的技术栈：

| 语言 | 仓库数 | 代表项目 |
|------|--------|----------|
| Go | 3 | pi-go, litellm-gateway, wechat-api |
| Python | 5 | easyagent, VibeCheck, knowledge-base-rd, vid-publisher, dabai-memory |
| TypeScript/JS | 3 | custom-skills (web), openhanako (fork), multimodal-transport |
| Shell | 5 | 各 memory 仓库的脚本 |
| 无代码/文档 | 8 | 配置、计划、简历等 |

**问题本质**：你在 **3 个月内接触了 5 种语言**，但每个项目的深度有限：
- pi-go 开发了 10 天
- easyagent 开发了 15 天
- litellm-gateway 开发了 6 周（最持久）
- 其余大多 1-2 周

没有哪个项目达到了"可以给别人用"的完成度。

**改进方案**：
- **选定 Go 作为主语言**（有 pi-go + litellm-gateway 基础，且有 go-learning 项目）
- 至少用 3 个月时间持续迭代 pi-go，不要频繁切方向
- 深度 > 广度：一个 1000 star 的项目 > 十个 0 star 的项目

---

### 🟡 问题 6：easyagent 的架构决策问题

easyagent 作为一个飞书原生 Agent 框架，有几个值得反思的点：

1. **OpenAI SDK 强耦合**：`engine.py` 硬编码 `OpenAI()` 客户端，没有 Provider 抽象。如果 pi-go 已经有了完善的 Provider 注册制，为什么不复用？
2. **15 天完成但已暂停**：说明可能遇到了架构瓶颈或需求变化
3. **私有仓库**：如果是为了学习/实验，公开反而能获得反馈

---

### 🟡 问题 7：记忆仓库的工程成熟度不足

jarvis-memory / friday-memory / dabai-memory / lobster-legion 构成了一个分布式 Agent 记忆系统，概念很先进，但：

1. **Shell 脚本为主**：sync.sh、scan-skills.py 都是简陋脚本，错误处理弱
2. **无测试**：同步逻辑零测试，rsync `--delete` 一个误操作就能清空数据
3. **命名不统一**：`memory/2026-05-21-1911.md` vs `memory/2026-05-12.md`，格式混乱
4. **DREAMS.md 反思质量低**：大量 "No grounded reflections emerged" 模板文本
5. **子 Agent 未初始化**：business-assistant 和 life-assistant 的 IDENTITY.md 都是模板

---

### 🟢 问题 8：CI/CD 不一致

| 仓库 | 有 CI | 有测试 | 有 Docker | PR 触发 CI |
|------|-------|--------|-----------|------------|
| pi-go | ✅ | ✅ (42 文件) | ❌ | ❌ (仅 main push) |
| easyagent | ✅ | ✅ (30 文件) | ❌ | 不确定 |
| litellm-gateway | ✅ | ✅ (6 文件) | ✅ | ❌ |
| custom-skills | ✅ | ✅ (21 Vitest) | ❌ | ✅ |
| wechat-api | ❌ | ✅ (3 文件) | ❌ | - |
| wechat-radar | ❌ | ❌ | ❌ | - |
| VibeCheck | ✅ | ❌ (404) | ❌ | - |

**问题**：pi-go 的 CI 仅在 push to main 触发，PR 合并前不跑测试——失去了 CI 的核心价值（防护栏）。

---

## 三、亮点与优势（你做得好的地方）

### ✅ 1. 架构设计能力突出

pi-go 的四层分层架构（Core → Platform → Application → Entry）是本次审计中**最亮眼的设计**：
- `runtime.Application` 接口解耦 Platform↔Application
- `operations.Operations` 接口支持本地/SSH 切换
- Provider 注册制 + 懒加载
- Tool 泛型 + 分区并行执行

这个架构水平远超"10 天项目"应有的水准。

### ✅ 2. 工程习惯好

- Conventional Commits 格式一致
- PR 驱动开发（pi-go 全部走 PR）
- ADR（架构决策记录）习惯
- Claude Code 深度集成（CLAUDE.md + skills + workflows）

### ✅ 3. 学习驱动而非功能驱动

从 pi 原版学习项目、go-learning-for-java-dev、knowledge-base-rd 可以看出你是**带着目的学习**，这比盲目刷课高效得多。

### ✅ 4. custom-skills 的上游同步管道

三路合并 + 自动 PR 的上游同步机制是专业水准的设计。

### ✅ 5. 龙虾军团的分布式 Agent 概念

3 个 Agent 跨 3 台服务器通过 Git 异步协作，记忆分层（短期 → 长期 → Dream），这个概念在 2026 年 3 月就实现了，非常前瞻。

---

## 四、改进路线图

### 🔥 立即行动（本周）

| # | 行动 | 原因 |
|---|------|------|
| 1 | **撤销泄露的 GitHub Token** | 安全红线 |
| 2 | **Archive 停滞仓库**：career-profile, pku-exam, growth-agent, vid-publisher, side-hustle-ideas, knowledge-base-rd, multimodal-transport, go-learning-for-java-dev | 减少噪音 |
| 3 | **给 pi-go 添加 MIT License** | 开源第一步 |

### 📋 短期（1-2 周）

| # | 行动 | 原因 |
|---|------|------|
| 4 | pi-go CI 改为 PR 触发 | 防护栏 |
| 5 | pi-go 添加 GitHub Topics + 英文 README | 可发现性 |
| 6 | 清理已合并的 feature 分支 | 仓库整洁 |
| 7 | 确定 1 个主力方向，暂停其他 | 聚焦 |

### 🎯 中期（1-3 个月）

| # | 行动 | 原因 |
|---|------|------|
| 8 | pi-go 达到 v1.0 可用状态 | 产出成果 |
| 9 | 给 litellm-gateway 添加 README 和 License | 如果还活跃 |
| 10 | 选择：继续 easyagent 还是迁移飞书能力到 pi-go | 减少重复 |
| 11 | 清理 memory 仓库的 git history 中的密钥 | 安全收尾 |

### 🏆 长期目标

**把 pi-go 做成你有影响力项目**：
- 目标：3 个月内 50+ stars
- 路径：完善文档 → 写技术博客 → 在社区分享 → 接受 PR
- 核心：Go 语言的 Agent 框架在中国开发者社区有差异化价值

---

## 五、各仓库详细评分

| 仓库 | 完成度 | 代码质量 | 文档 | 安全 | 综合 |
|------|--------|----------|------|------|------|
| **pi-go** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | **8/10** |
| **easyagent** | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | **7.5/10** |
| **litellm-gateway** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ | **7/10** |
| **custom-skills** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ | **7/10** |
| **VibeCheck** | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ | ⭐⭐ | **5/10** |
| **wechat-api** | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ | **5/10** |
| **jarvis-memory** | ⭐⭐⭐ | ⭐⭐ | ⭐⭐ | ⭐ | **4/10** |
| **lobster-legion** | ⭐⭐ | ⭐⭐ | ⭐⭐ | ⭐ | **3.5/10** |

---

## 六、一句话总结

**你是一个技术品味好、架构能力强、学习速度快的开发者，但最大的问题是"开太多坑、填太少"。** 3 个月 24 个仓库说明你好奇心旺盛，但产出集中度不够。建议：选定 pi-go 作为主线，用 3 个月时间做到社区可用，其余全部 archive。一个成功的开源项目比一百个半成品都有价值。
