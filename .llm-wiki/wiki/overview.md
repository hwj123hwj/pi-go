---
tags: [project-overview, architecture, agent-framework]
date: 2026-06-22
---

# Pi-Go 项目概览

> 用 Go 实现的通用 Agent 框架，定位为 **可扩展的 Agent 底座 + 可插拔的应用层**。
> 方向：从编程助手演进为 **通用个人助手**（[[personal-assistant-roadmap]]）。

## 架构：四层分层

```
Entrypoints（组装与入口）
    → Application（领域应用层，可插拔）
        → Platform（运行时平台层，领域无关）
            → Core（核心层，零领域知识）
```

**层间依赖规则**：Core 不依赖上层；Platform 只依赖 Core；Application 通过 `runtime.Application` 接口与 Platform 解耦；Entrypoints 组装所有依赖。

See [[four-layer-architecture]] for the Skills vs Application decision framework.

## 核心能力

| 能力 | 说明 | 状态 |
|------|------|------|
| 多 Provider | Anthropic / OpenAI / DeepV / Mock | ✅ |
| Agent 双层循环 | 外层 follow-up + 内层 tool call | ✅ |
| 8 个内置工具 | read / write / edit / bash / grep / find / ls / [[web-fetch-tool\|web_fetch]] | ✅ |
| 工具生命周期 | Before/After hook + PrepareArguments + Confirmation | ✅ |
| 会话持久化 | JSONL append-only，树状分支 | ✅ |
| 上下文压缩 | 两级：[[micro-compact\|MicroCompact]]（无 LLM）+ LLM 摘要 | ✅ |
| 扩展系统 | Extension 接口支持工具/命令/事件钩子 | ✅ |
| 技能系统 | `.claude/skills/` 目录加载 SKILL.md | ✅ |
| Goal-Driven Loop | 目标驱动，LLM 评估器 + 关键词回退 | ✅ |
| 循环检测 | SHA256 指纹，连续相同工具调用检测 | ✅ |
| 确认门控 | 危险工具执行前用户确认 | ✅ |
| SSH 远程执行 | 通过 Operations 抽象切换 | ✅ |
| Slash Commands | 15 个内置命令 | ✅ |
| HTTP API | REST + SSE + WebSocket | ✅ |
| 桌面客户端 | Electron + React ([[desktop-app]]) | ✅ |
| 飞书桥接 | 独立服务接入飞书群聊 ([[feishu-integration]]) | ✅ |
| [[music-agent\|音乐助手]] | 多源音乐（Bilibili 为主力 + NetEase 推荐），6 个专用工具，质量过滤 | ✅ |
| [[kb-agent\|知识库助手]] | 个人知识库检索（agent-lessons），3 个只读工具 | ✅ |
| 外部工具 | HTTP 回调注册 ([[external-tools]]) | ✅ |
| AI Agent 指导 | CLAUDE.md / AGENTS.md 编码规范 ([[agent-guidance-system]]) | ✅ |

## 三种交付入口

1. **CLI** — 交互式/单次/服务模式
2. **Desktop** — Electron + React GUI ([[desktop-app]])
3. **Server + Feishu** — 飞书群聊 Agent ([[feishu-integration]])

## 三种应用层

1. **Coding Agent** — 代码编辑助手（主要应用）([[coding-application]])
2. **Music Agent** — 多源音乐助手（Bilibili 播放主力 + NetEase 推荐，质量过滤，跨源降级，全局播放器）([[music-agent]])
3. **KB Agent** — 个人知识库检索（507 张知识卡片 + 38 个项目日志，3 个只读工具）([[kb-agent]])

## 技术栈

- **语言**：Go 1.24+
- **桌面端**：Electron 33 / React 19 / Vite 6 / TypeScript 5 / Zustand
- **外部依赖**：极简（标准库为主；gorilla/websocket, larksuite oapi-sdk, lipgloss）
- **存储**：文件系统（JSONL 会话 + JSON meta）
- **代码量**：~14,000 行 Go + 54 个测试文件 + ~4,000 行 TypeScript/React

## 竞争力分析

See [[competitive-analysis]] for detailed feature gap analysis vs DeepVcodeClient.

## 项目状态

- **分支**：main
- **路线图阶段**：Phase 0-1 completed, Phase 2-5 in progress
- **近期方向**：[[personal-assistant-roadmap\|通用个人助手]]、工具增强、Desktop 打磨
- **部署**：GitHub Actions CI/CD → Ubuntu server ([[deployment-infrastructure]])
