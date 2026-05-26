---
status: draft
author: weijian
created: 2026-05-26
---

# Web-Access 浏览器操控能力规划

> 目标：为 pi-go Agent 提供内置的浏览器操控能力，复用用户日常 Chrome（天然携带登录态），
> 同时吸收 agent-browser 的无障碍树 snapshot 等优秀设计。
> 以 Go package 形式集成到 pi-go，可选 CLI 包装独立使用，不引入 MCP 中间层。

## 1. 背景

### 现状

当前 pi-go Agent 的联网能力仅有 WebSearch / WebFetch / curl（通过 bash 工具），存在以下限制：

- **无登录态**：无法访问需要登录的站点（内部系统、飞书、Notion 等）
- **静态层限制**：SPA、反爬站点（小红书、微信公众号）的静态抓取拿不到有效内容
- **无交互能力**：不能点击、翻页、填表、提交

### 参考项目

| 项目 | Star | 核心思路 | 与本项目的关键差异 |
|------|------|----------|-------------------|
| web-access skill（当前使用） | — | CDP Proxy 直连用户 Chrome，HTTP API 操控 | Node.js 实现，需要 Node 22+；通过 curl 调用 |
| [agent-browser](https://github.com/vercel-labs/agent-browser)（Vercel） | 34.3k | Rust CLI，启动独立 Chrome for Testing | 不复用用户 Chrome，无登录态；无障碍树 snapshot 设计优秀 |
| [chrome-devtools-mcp](https://github.com/ChromeDevTools/chrome-devtools-mcp)（Google） | 41.8k | MCP Server + Puppeteer，启动独立 Chrome | 面向 Web 开发调试（性能/内存/Lighthouse）；MCP 架构对个人 Agent 过重 |

### 设计决策

1. **复用用户 Chrome，不启动新浏览器** — 个人 Agent 的核心价值是"像你一样上网"，登录态不可替代
2. **Go package + 可选 CLI，不用 MCP** — 个人场景直接函数调用最干净，不需要协议中间层
3. **吸收 agent-browser 的无障碍树 snapshot** — 比 CSS 选择器 + eval 的方式对 Agent 更友好
4. **不做 iOS Safari / 多浏览器引擎** — 不是个人 Agent 的核心需求，保持范围收敛

## 2. 架构

```
┌─────────────────────────────────────────────────┐
│  pi-go Agent                                     │
│                                                   │
│  ┌─────────────┐    ┌──────────────────────────┐ │
│  │ Agent Session│───▶│ webaccess package         │ │
│  │              │    │                            │ │
│  │ 通过 Tool 或  │    │  ┌──────────────────────┐ │ │
│  │ 直接调用      │    │  │ Browser (连接管理)    │ │ │
│  │              │    │  │  - 端口发现            │ │ │
│  └─────────────┘    │  │  - WebSocket 连接      │ │ │
│                      │  │  - Tab 生命周期        │ │ │
│                      │  └────────┬─────────────┘ │ │
│                      │           │                │ │
│                      │  ┌────────▼─────────────┐ │ │
│                      │  │ Tab (单个页面操控)     │ │ │
│                      │  │  - Navigate / Back    │ │ │
│                      │  │  - Snapshot (AX 树)   │ │ │
│                      │  │  - Eval (JS 执行)     │ │ │
│                      │  │  - Click / ClickAt    │ │ │
│                      │  │  - Fill / Type        │ │ │
│                      │  │  - Scroll             │ │ │
│                      │  │  - Screenshot         │ │ │
│                      │  │  - Wait (等待策略)     │ │ │
│                      │  └──────────────────────┘ │ │
│                      └──────────────────────────┘ │
└─────────────────────────────────────────────────┘
                        │
                        │ WebSocket (CDP)
                        ▼
                用户日常 Chrome
                （天然携带登录态）
```

### 核心类型

```go
// Browser 管理与 Chrome 的连接，负责端口发现和 WebSocket 通信
type Browser struct { ... }

// Tab 代表一个浏览器标签页，提供所有页面操控方法
type Tab struct { ... }

// SnapshotResult 是无障碍树快照结果
type SnapshotResult struct {
    Elements []AXNode  // 扁平化的可交互元素列表
    RefMap   map[string]int  // "@e1" -> backendNodeId 映射
}

// AXNode 是简化后的无障碍树节点
type AXNode struct {
    Ref     string   // "@e1", "@e2" ...
    Role    string   // button, link, textbox, heading ...
    Name    string   // 可访问名称
    Value   string   // 当前值（如有）
    Visible bool     // 是否可见
}
```

## 3. 能力范围

### 第一阶段：核心能力（MVP）

从 web-access skill 平移的能力，用 Go 重新实现：

| 能力 | API | CDP 对应 |
|------|-----|----------|
| 连接管理 | `NewBrowser()` / `Connect()` | 端口发现 + WebSocket |
| 创建后台 Tab | `browser.NewTab(url)` | `Target.createTarget` |
| 关闭 Tab | `tab.Close()` | `Target.closeTarget` |
| 页面导航 | `tab.Navigate(url)` | `Page.navigate` |
| 执行 JS | `tab.Eval(expr)` | `Runtime.evaluate` |
| 点击元素 | `tab.Click(selector)` | `Runtime.evaluate` (el.click) |
| 真实鼠标点击 | `tab.ClickAt(selector)` | `Input.dispatchMouseEvent` |
| 滚动 | `tab.Scroll(y)` | `Runtime.evaluate` |
| 截图 | `tab.Screenshot(file)` | `Page.captureScreenshot` |
| 页面信息 | `tab.Info()` | `Runtime.evaluate` |
| 反风控 | 内置 | `Fetch.enable` 拦截调试端口探测 |

### 第二阶段：无障碍树 Snapshot（从 agent-browser 吸收）

| 能力 | API | 说明 |
|------|-----|------|
| 完整快照 | `tab.Snapshot()` | 返回简化后的无障碍树 + @e 引用 |
| 交互元素快照 | `tab.SnapshotInteractive()` | 只返回可交互元素 |
| 引用定位操作 | `tab.ClickRef("@e3")` | 通过 snapshot 引用点击 |
| 引用填写 | `tab.FillRef("@e5", "text")` | 通过 snapshot 引用填写 |

Snapshot 输出示例：

```
[1] link "首页"
[2] link "热门"
[3] textbox "搜索"
[4] button "搜索"
[5] heading "今日推荐"
[6] article "Go 1.24 发布..."
[7] link "阅读全文"
```

Agent 不需要 CSS 选择器，直接 `ClickRef("@7")` 即可。

### 第三阶段：增强能力（按需补齐）

| 能力 | API | CDP 对应 |
|------|-----|----------|
| 等待策略 | `tab.WaitFor(selector)` | `Runtime.evaluate` 轮询 + `Page.lifecycleEvent` |
| 表单填写 | `tab.Fill(selector, value)` | `Runtime.evaluate` |
| 键盘输入 | `tab.Press(key)` | `Input.dispatchKeyEvent` |
| 拖拽 | `tab.Drag(src, dst)` | `Input.dispatchMouseEvent` 序列 |
| 文件上传 | `tab.Upload(selector, files)` | `DOM.setFileInputFiles` |
| Cookie 操作 | `browser.Cookies()` | `Network.getAllCookies` |
| 网络拦截 | `tab.Intercept(pattern)` | `Fetch.enable` + `Fetch.requestPaused` |
| 设备模拟 | `tab.SetViewport(w, h)` | `Emulation.setDeviceMetricsOverride` |
| PDF 导出 | `tab.PrintPDF(file)` | `Page.printToPDF` |
| 截图标注 | `tab.ScreenshotAnnotated(file)` | 截图 + 在元素位置画标注框 |
| 批量执行 | `browser.Batch(commands)` | 应用层逻辑 |

### 明确不做

| 能力 | 原因 |
|------|------|
| iOS Safari 控制 | 走 WebDriver/Appium 协议，非 CDP，工程量大且非核心需求 |
| 多浏览器引擎（lightpanda 等） | 不是个人 Agent 的核心场景 |
| MCP 协议层 | 个人使用直接函数调用，不需要协议中间层 |
| 独立浏览器启动 | 核心设计决策：复用用户 Chrome |

## 4. 包结构

```
internal/webaccess/
├── browser.go       # Browser 类型：连接管理、端口发现、WebSocket 通信
├── tab.go           # Tab 类型：页面操控方法
├── snapshot.go      # 无障碍树提取与简化
├── cdp.go           # CDP 协议底层：sendCommand、事件监听、session 管理
├── discover.go      # Chrome 调试端口发现（DevToolsActivePort + 常见端口扫描）
├── guard.go         # 反风控：拦截页面对调试端口的探测
├── types.go         # 公共类型定义
├── browser_test.go
├── tab_test.go
├── snapshot_test.go
└── cdp_test.go
```

## 5. 使用方式

### 作为 pi-go 内部 package

```go
// 在 Agent 的 tool 实现中直接调用
import "github.com/earendil-works/pi-go/internal/webaccess"

browser, _ := webaccess.NewBrowser()
tab, _ := browser.NewTab("https://example.com")
defer tab.Close()

// 方式一：传统 CSS 选择器 + eval
title, _ := tab.Eval("document.title")

// 方式二：无障碍树 snapshot（推荐）
snap, _ := tab.SnapshotInteractive()
// snap.Elements = [{Ref:"@e1", Role:"button", Name:"搜索"}, ...]
tab.ClickRef("@e1")
```

### 作为独立 CLI（可选）

```bash
# 单次命令模式
webaccess open https://example.com --eval "document.title"

# 常驻 HTTP 模式（兼容现有 skill 的 curl 调用方式）
webaccess serve --port 3456
```

## 6. 关键实现细节

### 端口发现

复用 web-access skill 的策略：
1. 读取 `DevToolsActivePort` 文件（macOS: `~/Library/Application Support/Google/Chrome/DevToolsActivePort`）
2. 文件第一行是端口号，第二行是 WebSocket 路径
3. 回退扫描常见端口：9222、9229、9333

### 无障碍树 Snapshot 的处理

CDP `Accessibility.getFullAXTree` 返回的原始数据包含大量噪音，需要：
1. **过滤无意义节点**：移除 `generic`、`presentation`、`Ignored` 节点
2. **线性化**：将树结构拍平为有序列表，只保留有意义的叶子/交互节点
3. **编号映射**：`@e1`、`@e2`... 映射到 `backendDOMNodeId`，后续操作通过 `DOM.resolveNode` → `Runtime.callFunctionOn` 定位
4. **交互元素筛选**：只保留 role 为 button、link、textbox、checkbox 等可交互类型的节点

### Tab 生命周期

- 所有 Agent 创建的 tab 通过 `Target.createTarget` 以 `background: true` 方式打开
- 维护 `managedTabs` 集合，任务结束后全部关闭
- 闲置超时自动清理（默认 15 分钟）
- 绝对不操作用户已有的 tab

### 反风控

- 连接 tab 时自动启用 `Fetch.enable`，拦截页面对 `127.0.0.1:{chromePort}` 的探测请求
- 避免网站通过检测 Chrome 调试端口发现自动化行为

## 7. 实施路径

```
Phase 1 — 核心能力（MVP）
├── cdp.go: CDP WebSocket 通信层
├── discover.go: 端口发现
├── browser.go: Browser 类型（连接、Tab 管理）
├── tab.go: Tab 基础操作（NewTab, Close, Navigate, Eval, Click, Scroll, Screenshot, Info）
├── guard.go: 反风控拦截
└── 测试覆盖

Phase 2 — 无障碍树 Snapshot
├── snapshot.go: AX 树提取 + 简化 + @e 引用
├── tab.go 补充: ClickRef, FillRef, SnapshotInteractive
└── 真实网站打磨 snapshot 输出质量

Phase 3 — 增强能力（按需）
├── 等待策略、表单填写、键盘输入
├── Cookie/网络拦截/设备模拟
├── 截图标注、批量执行
└── 可选 CLI 包装
```

## 8. 依赖

| 依赖 | 用途 | 是否已有 |
|------|------|----------|
| `nhooyr.io/websocket` | WebSocket 客户端（连接 Chrome CDP） | ✅ go.mod 已有 |
| Go 标准库 `net/http` | 可选的 HTTP server 模式 | ✅ 内置 |
| Go 标准库 `image` + `image/draw` | 截图标注（Phase 3） | ✅ 内置 |

**零外部新增依赖**。WebSocket 库已经在项目的 go.mod 中。

## 9. 风险与缓解

| 风险 | 缓解 |
|------|------|
| Chrome CDP 接口在不同版本间有差异 | 只使用稳定的高频 CDP 域（Runtime、Page、Target、Input、Accessibility），避免实验性 API |
| 无障碍树在不同网站上质量差异大 | Snapshot 同时保留 Eval 作为备选；Agent 可以在 snapshot 不够用时回退到 CSS 选择器 |
| 用户 Chrome 未开启远程调试 | 首次使用时检测并给出明确引导（同 web-access skill 的 check-deps 逻辑） |
| 并发 tab 操作竞态 | 每个 Tab 独立 sessionId，Browser 层串行化 CDP 连接管理 |
