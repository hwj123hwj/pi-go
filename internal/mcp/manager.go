package mcp

// manager.go — MCP 服务器管理器。
//
// 负责多个 MCP 服务器的生命周期管理：注册、连接、工具发现、健康检查、
// 崩溃重连、优雅关闭。对应 hwjcode mcp-client.ts 中的 discoverMcpTools /
// connectAndDiscover / unloadMcpServer 等编排逻辑。

import (
	"context"
	"fmt"
	"sync"
	"time"

	"log/slog"

	"github.com/hwj123hwj/pi-go/sdk/agent"
)

// ---------------------------------------------------------------------------
// ServerStatus
// ---------------------------------------------------------------------------

// ServerStatus 描述单个 MCP 服务器的连接状态。
type ServerStatus string

const (
	StatusDisconnected ServerStatus = "disconnected"
	StatusConnecting   ServerStatus = "connecting"
	StatusConnected    ServerStatus = "connected"
)

// ---------------------------------------------------------------------------
// managedServer
// ---------------------------------------------------------------------------

// managedServer 是 Manager 内部跟踪的单个服务器状态。
type managedServer struct {
	config   MCPServerConfig
	client   *Client
	status   ServerStatus
	tools    []agent.Tool // 已发现的工具适配器
	failures int         // 连续失败次数（用于退避）
}

// ---------------------------------------------------------------------------
// Manager
// ---------------------------------------------------------------------------

// Manager 管理多个 MCP 服务器连接和工具发现。
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*managedServer

	// 重连配置
	maxRetries    int           // 最大重试次数（0 = 不自动重连）
	retryInterval time.Duration // 重试间隔
}

// NewManager 创建一个新的 MCP 管理器。
func NewManager() *Manager {
	return &Manager{
		servers:       make(map[string]*managedServer),
		maxRetries:    3,
		retryInterval: 5 * time.Second,
	}
}

// SetRetryPolicy 配置自动重连策略。
func (m *Manager) SetRetryPolicy(maxRetries int, interval time.Duration) {
	m.maxRetries = maxRetries
	m.retryInterval = interval
}

// ---------------------------------------------------------------------------
// 注册 / 列表
// ---------------------------------------------------------------------------

// RegisterServer 注册一个 MCP 服务器配置（尚不连接）。
// 如果同名服务器已存在，覆盖其配置。
func (m *Manager) RegisterServer(name string, cfg MCPServerConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("server %q: %w", name, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[name] = &managedServer{
		config: cfg,
		status: StatusDisconnected,
	}
	slog.Info("MCP server registered", "server", name, "transport", cfg.Transport())
	return nil
}

// RegisterServers 批量注册服务器配置。
func (m *Manager) RegisterServers(servers map[string]MCPServerConfig) error {
	for name, cfg := range servers {
		if err := m.RegisterServer(name, cfg); err != nil {
			return err
		}
	}
	return nil
}

// UnregisterServer 移除一个服务器（会先断开连接）。
func (m *Manager) UnregisterServer(ctx context.Context, name string) error {
	m.mu.Lock()
	srv, ok := m.servers[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("server %q not found", name)
	}
	delete(m.servers, name)
	m.mu.Unlock()

	if srv.client != nil {
		_ = srv.client.Close()
	}
	slog.Info("MCP server unregistered", "server", name)
	return nil
}

// ListServers 返回所有已注册的服务器名。
func (m *Manager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

// GetStatus 返回指定服务器的当前状态。
func (m *Manager) GetStatus(name string) (ServerStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	srv, ok := m.servers[name]
	if !ok {
		return StatusDisconnected, false
	}
	return srv.status, true
}

// AllStatuses 返回所有服务器的状态快照。
func (m *Manager) AllStatuses() map[string]ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]ServerStatus, len(m.servers))
	for name, srv := range m.servers {
		result[name] = srv.status
	}
	return result
}

// ---------------------------------------------------------------------------
// 连接 / 发现
// ---------------------------------------------------------------------------

// ConnectAll 连接所有已注册但尚未连接的服务器，并发现工具。
// 单个服务器失败不会中断其他服务器的连接。
func (m *Manager) ConnectAll(ctx context.Context) []error {
	m.mu.RLock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	m.mu.RUnlock()

	var errs []error
	for _, name := range names {
		if err := m.ConnectOne(ctx, name); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// ConnectOne 连接指定服务器并发现工具。
func (m *Manager) ConnectOne(ctx context.Context, name string) error {
	m.mu.Lock()
	srv, ok := m.servers[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("server %q not found", name)
	}
	if srv.status == StatusConnected {
		m.mu.Unlock()
		return nil // 已连接
	}
	srv.status = StatusConnecting
	m.mu.Unlock()

	cfg := srv.config
	client := NewClient(name)
	if cfg.Timeout > 0 {
		client.SetTimeout(time.Duration(cfg.Timeout) * time.Millisecond)
	}

	slog.Info("MCP connecting", "server", name, "transport", cfg.Transport())
	if err := client.Connect(ctx, cfg); err != nil {
		m.mu.Lock()
		srv.status = StatusDisconnected
		srv.failures++
		m.mu.Unlock()
		return fmt.Errorf("connect %q: %w", name, err)
	}

	// 发现工具
	tools, err := m.discoverTools(ctx, name, client, cfg)
	if err != nil {
		_ = client.Close()
		m.mu.Lock()
		srv.status = StatusDisconnected
		srv.failures++
		m.mu.Unlock()
		return fmt.Errorf("discover tools from %q: %w", name, err)
	}

	m.mu.Lock()
	srv.client = client
	srv.tools = tools
	srv.status = StatusConnected
	srv.failures = 0
	m.mu.Unlock()

	slog.Info("MCP connected", "server", name, "tools", len(tools))
	return nil
}

// discoverTools 从已连接的客户端发现工具，并应用 include/exclude 过滤。
func (m *Manager) discoverTools(ctx context.Context, serverName string, client *Client, cfg MCPServerConfig) ([]agent.Tool, error) {
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		return nil, err
	}

	timeout := DefaultTimeout
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Millisecond
	}

	var tools []agent.Tool
	for _, mt := range mcpTools {
		if !isToolEnabled(mt.Name, cfg) {
			slog.Debug("MCP tool filtered", "server", serverName, "tool", mt.Name)
			continue
		}
		adapter := NewMCPToolAdapter(client, serverName, mt, timeout)
		tools = append(tools, adapter)
	}
	return tools, nil
}

// isToolEnabled 根据 includeTools / excludeTools 判断工具是否应注册。
func isToolEnabled(toolName string, cfg MCPServerConfig) bool {
	// excludeTools 优先级更高
	for _, ex := range cfg.ExcludeTools {
		if ex == toolName {
			return false
		}
	}
	if len(cfg.IncludeTools) == 0 {
		return true
	}
	for _, inc := range cfg.IncludeTools {
		if inc == toolName {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 工具获取
// ---------------------------------------------------------------------------

// AllTools 返回所有已连接服务器发现的工具列表。
// 这是将 MCP 工具注册到 agent 工具注册表的主要入口。
func (m *Manager) AllTools() []agent.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var tools []agent.Tool
	for _, srv := range m.servers {
		if srv.status == StatusConnected {
			tools = append(tools, srv.tools...)
		}
	}
	return tools
}

// ToolsForServer 返回指定服务器的工具列表。
func (m *Manager) ToolsForServer(name string) []agent.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	srv, ok := m.servers[name]
	if !ok || srv.status != StatusConnected {
		return nil
	}
	return srv.tools
}

// ---------------------------------------------------------------------------
// 健康检查 / 重连
// ---------------------------------------------------------------------------

// HealthCheck 检查指定服务器是否仍然连接。
// 通过发送一个轻量请求（tools/list）来验证。
func (m *Manager) HealthCheck(ctx context.Context, name string) bool {
	m.mu.RLock()
	srv, ok := m.servers[name]
	m.mu.RUnlock()
	if !ok || srv.client == nil {
		return false
	}

	// 尝试列出工具作为存活探测
	_, err := srv.client.ListTools(ctx)
	if err != nil {
		slog.Warn("MCP health check failed", "server", name, "err", err)
		m.mu.Lock()
		srv.status = StatusDisconnected
		m.mu.Unlock()
		return false
	}
	return true
}

// Reconnect 尝试重新连接指定服务器。
func (m *Manager) Reconnect(ctx context.Context, name string) error {
	m.mu.Lock()
	srv, ok := m.servers[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("server %q not found", name)
	}
	// 关闭旧连接
	if srv.client != nil {
		_ = srv.client.Close()
		srv.client = nil
	}
	srv.status = StatusDisconnected
	m.mu.Unlock()

	return m.ConnectOne(ctx, name)
}

// ReconnectIfDisconnected 检查并重连所有断开的服务器（用于自动恢复）。
func (m *Manager) ReconnectIfDisconnected(ctx context.Context) {
	m.mu.RLock()
	disconnected := make([]string, 0)
	for name, srv := range m.servers {
		if srv.status == StatusDisconnected && srv.failures < m.maxRetries {
			disconnected = append(disconnected, name)
		}
	}
	m.mu.RUnlock()

	for _, name := range disconnected {
		slog.Info("MCP auto-reconnect", "server", name)
		if err := m.Reconnect(ctx, name); err != nil {
			slog.Warn("MCP auto-reconnect failed", "server", name, "err", err)
		}
	}
}

// ---------------------------------------------------------------------------
// 关闭
// ---------------------------------------------------------------------------

// Close 关闭所有服务器连接。
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for name, srv := range m.servers {
		if srv.client != nil {
			if err := srv.client.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		srv.status = StatusDisconnected
		slog.Info("MCP server closed", "server", name)
	}
	return firstErr
}
