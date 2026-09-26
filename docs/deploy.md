# 自动部署说明

本文档对应当前仓库内的自动部署方案：

- GitHub Actions 负责测试、构建、上传和重启
- 服务器系统为 Ubuntu
- 使用 `systemd` 托管进程
- EasyAgent 服务仅监听 `127.0.0.1:8081`；服务器的 `8080` 已由 Nginx 使用
- 当前服务器继续使用既有部署目录 `/opt/pi-go` 和 systemd 单元 `pi-go`，以兼容已部署环境

## 部署结构

服务器目录结构：

```text
/opt/pi-go/
├── current -> /opt/pi-go/releases/release-<git-sha>
├── releases/
│   └── release-<git-sha>/
│       ├── easyagent
│       ├── easyagent-bridge
│       ├── scripts/feishu-bridge-configured.sh
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
- `PI_GO_ENV`（现有 GitHub Actions secret 名称，为兼容部署保留）
  值：服务器 `.env` 文件内容，示例见下文

## 推荐环境变量

`PI_GO_ENV` 建议至少包含以下使用新前缀的变量：

```dotenv
EA_PROVIDER=openai
OPENAI_API_KEY=your_key_here
OPENAI_MODEL=gpt-4o-mini
OPENAI_BASE_URL=https://api.openai.com/v1

EA_HOST=127.0.0.1
EA_PORT=8081
EA_ENABLE_BASH=false
EA_SESSION_FILE=/opt/pi-go/shared/data/session.jsonl
PI_AGENT_URL=http://127.0.0.1:8081
```

如果你们后面切换到 Anthropic，对应替换成：

```dotenv
EA_PROVIDER=anthropic
ANTHROPIC_API_KEY=your_key_here
ANTHROPIC_MODEL=claude-sonnet-4-5
ANTHROPIC_BASE_URL=https://api.anthropic.com

EA_HOST=127.0.0.1
EA_PORT=8081
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

- [.github/workflows/deploy.yml](https://github.com/hwj123hwj/easyagent/blob/main/.github/workflows/deploy.yml)

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
8. 重启已配置的飞书桥接服务；缺少凭据时跳过启动，不进入重启循环
9. 对 `http://127.0.0.1:8081/health` 做健康检查

## 手动查看服务

部署后可在服务器上查看：

```bash
systemctl status pi-go
journalctl -u pi-go -n 200 --no-pager
curl http://127.0.0.1:8081/health
```

## 飞书接入建议

服务器部署后，在同一个账号下完成 `/feishu setup`，再执行 `/feishu start` 启动桥接服务。完成扫码的账号会收到欢迎语和权限提示；手动配置凭据时，需要设置 `FEISHU_OWNER_OPEN_ID`。

当前方案适合以下形态：

- agent 服务仅作为本机内部 HTTP 服务
- 飞书适配层与 `EasyAgent` 运行在同一台机器
- 飞书适配层通过 `127.0.0.1:8081` 调用 agent

桥接服务通过长连接接收事件，不需要额外暴露公网端口。
