---
tags: [project-overview, architecture, agent-framework]
date: 2026-06-14
---

# Pi-Go 项目概览

> 用 Go 实现的通用 Agent 框架，定位为 **可扩展的 Agent 底座 + 可插拔的应用层**。
> 当前主要应用层：coding-agent（代码编辑助手）。

## 架构：四层分层

```
Entrypoints（组装与入口）
    → Application（领域应用层，可插拔）
        → Platform（运行时平台层，领域无关）
            → Core（核心层，零领域知识）
```

**层间依赖规则**：Core 不依赖上层；Platform 只依赖 Core；Application 通过 `runtime.Application` 接口与 Platform 解耦；Entrypoints 组装所有依赖。

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
| Slash Commands | 14 个内置命令 | ✅ |
| HTTP API | REST + SSE + WebSocket | ✅ |
| 桌面客户端 | Electron + React (v0.2.0) | ✅ 可用 |
| 飞书桥接 | 独立服务接入飞书群聊 | ✅ 基础版 |
| [[music-agent\|音乐助手]] | NetEase 云音乐，6 个专用工具 | ✅ |
| 外部工具 | HTTP 回调注册 | ✅ |

## 三种交付入口

1. **CLI** — 交互式/单次/服务模式
2. **Desktop** — Electron + React GUI ([[desktop-app]])
3. **Server + Feishu** — 飞书群聊 Agent ([[feishu-integration]])

## 两种应用层

1. **Coding Agent** — 代码编辑助手（主要应用）
2. **Music Agent** — 音乐助手，基于 NetEase 云音乐（[[music-agent]]）

## 技术栈

- **语言**：Go 1.24+
- **桌面端**：Electron 33 / React 19 / Vite 6 / TypeScript 5 / Zustand
- **外部依赖**：极简（标准库为主；gorilla/websocket, larksuite oapi-sdk, lipgloss）
- **存储**：文件系统（JSONL 会话 + JSON meta）
- **代码量**：~12,400 行 Go + 39 个测试文件 + ~4,000 行 TypeScript/React

## 项目状态

- **分支**：main
- **路线图阶段**：Phase 1（基础能力已完成），Phase 2-5 规划中
- **近期方向**：Desktop 打磨、工具增强、Agent 协作