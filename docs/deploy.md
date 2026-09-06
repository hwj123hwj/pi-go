# 自动部署说明

本文档对应当前仓库内的自动部署方案：

- GitHub Actions 负责测试、构建、上传和重启
- 服务器系统为 Ubuntu
- 使用 `systemd` 托管进程
- 服务仅监听 `127.0.0.1:8080`
- 默认部署目录为 `/opt/pi-go`

## 部署结构

服务器目录结构：

```text
/opt/pi-go/
├── current -> /opt/pi-go/releases/release-<git-sha>
├── releases/
│   └── release-<git-sha>/
│       ├── pi-agent
│       └── README.md
└── shared/
    └── .env
```

## GitHub Secrets

需要在仓库 `Settings -> Secrets and variables -> Actions` 中创建以下 secrets：

- `DEPLOY_HOST`
  值：`8.141.97.21`
- `DEPLOY_USER`
  值：`root`
- `DEPLOY_PORT`
  值：`22`
- `DEPLOY_PATH`
  值：`/opt/pi-go`
- `DEPLOY_SSH_KEY`
  值：本地 `~/.ssh/id_cloud` 私钥完整内容
- `PI_GO_ENV`
  值：服务器 `.env` 文件内容，示例见下文

## 推荐环境变量

`PI_GO_ENV` 建议至少包含：

```dotenv
PI_GO_PROVIDER=openai
OPENAI_API_KEY=your_key_here
OPENAI_MODEL=gpt-4o-mini
OPENAI_BASE_URL=https://api.openai.com/v1

PI_GO_HOST=127.0.0.1
PI_GO_PORT=8080
PI_GO_ENABLE_BASH=false
PI_GO_SESSION_FILE=/opt/pi-go/shared/data/session.jsonl
```

如果你们后面切换到 Anthropic，对应替换成：

```dotenv
PI_GO_PROVIDER=anthropic
ANTHROPIC_API_KEY=your_key_here
ANTHROPIC_MODEL=claude-sonnet-4-5
ANTHROPIC_BASE_URL=https://api.anthropic.com

PI_GO_HOST=127.0.0.1
PI_GO_PORT=8080
```

## 首次部署前准备

首次部署前，建议先在服务器执行：

```bash
mkdir -p /opt/pi-go/shared/data
```

然后确认系统带有：

```bash
systemctl --version
curl --version
```

## 工作流行为

工作流文件位于：

- [.github/workflows/deploy.yml](https://github.com/hwj123hwj/pi-go/blob/main/.github/workflows/deploy.yml)

触发方式：

- push 到 `main`
- 手动执行 `workflow_dispatch`

执行步骤：

1. `go test ./...`
2. 构建 `linux/amd64` 二进制
3. 打包 release tarball
4. 通过 SSH/SCP 上传到服务器 `/tmp`
5. 渲染并安装 `systemd` service
6. 更新 `current` 软链
7. `systemctl restart pi-go`
8. 对 `http://127.0.0.1:8080/health` 做健康检查

## 手动查看服务

部署后可在服务器上查看：

```bash
systemctl status pi-go
journalctl -u pi-go -n 200 --no-pager
curl http://127.0.0.1:8080/health
```

## 飞书接入建议

如果最终是“飞书套 agent 的壳”，当前方案适合以下形态：

- agent 服务仅作为本机内部 HTTP 服务
- 飞书适配层与 `pi-go` 运行在同一台机器
- 飞书适配层通过 `127.0.0.1:8080` 调用 agent

如果后面改成长连接事件模式，也可以继续沿用这套部署方式，不需要额外暴露公网端口。
