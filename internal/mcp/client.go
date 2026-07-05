package mcp

// client.go — MCP 客户端：通过 JSON-RPC 2.0 与 MCP 服务器通信。
//
// 支持两种传输：
//   - stdio：启动子进程，通过 stdin/stdout 管道收发换行分隔的 JSON-RPC 消息。
//   - SSE：通过 HTTP POST 发送请求，通过 Server-Sent Events 接收响应。
//
// 核心 API：
//   - Connect(ctx)     — 建立连接并完成 initialize 握手
//   - ListTools(ctx)   — tools/list
//   - CallTool(ctx, …) — tools/call
//   - ListResources / ReadResource / ListPrompts
//   - Close()          — 关闭连接
//
// 参考 hwjcode: packages/core/src/tools/mcp-client.ts

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"
)

// ---------------------------------------------------------------------------
// 超时常量
// ---------------------------------------------------------------------------

const (
	// DefaultTimeout 是 MCP 请求的默认超时（10 分钟，与 hwjcode 一致）。
	DefaultTimeout = 10 * 60 * time.Second

	// ConnectTimeout 是建立连接（initialize 握手）的超时。
	ConnectTimeout = 30 * time.Second
)

// ---------------------------------------------------------------------------
// Transport 接口
// ---------------------------------------------------------------------------

// Transport 抽象底层通信通道。Client 通过它发送和接收 JSON-RPC 消息。
type Transport interface {
	// Send 写入一行 JSON-RPC 消息（请求 / 通知 / 响应）。
	Send(ctx context.Context, data []byte) error
	// Recv 读出一条 JSON-RPC 消息（阻塞直到有数据或出错）。
	Recv(ctx context.Context) ([]byte, error)
	// Close 关闭传输通道。
	Close() error
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client 是一个连接到 MCP 服务器的客户端实例。
type Client struct {
	serverName string
	transport  Transport
	timeout    time.Duration

	mu       sync.Mutex
	nextID   int64
	pending  map[int64]chan *rpcResult
	closed   bool
	closeErr error

	// initialize 握手后填充
	serverInfo      ClientInfo
	serverCaps      ServerCapabilities
	initialized     bool
}

// rpcResult 是一个待处理的 JSON-RPC 响应载体。
type rpcResult struct {
	resp  *JSONRPCResponse
	err   *JSONRPCErrorResponse
	parse error
}

// NewClient 创建一个未连接的 MCP 客户端。
// Connect() 会建立实际传输并完成握手。
func NewClient(serverName string) *Client {
	return &Client{
		serverName: serverName,
		timeout:    DefaultTimeout,
		pending:    make(map[int64]chan *rpcResult),
	}
}

// ServerName 返回服务器名称。
func (c *Client) ServerName() string { return c.serverName }

// ServerCapabilities 返回 initialize 握手后服务器报告的能力（握手前为零值）。
func (c *Client) ServerCapabilities() ServerCapabilities { return c.serverCaps }

// ServerInfo 返回 initialize 握手后服务器报告的 info。
func (c *Client) ServerInfo() ClientInfo { return c.serverInfo }

// SetTimeout 设置后续请求的默认超时。
func (c *Client) SetTimeout(d time.Duration) {
	c.timeout = d
}

// ---------------------------------------------------------------------------
// 连接建立
// ---------------------------------------------------------------------------

// Connect 根据 config 建立传输并完成 initialize 握手。
func (c *Client) Connect(ctx context.Context, cfg MCPServerConfig) error {
	transport, err := createTransport(c.serverName, cfg)
	if err != nil {
		return err
	}
	c.transport = transport

	// 启动读取循环
	go c.readLoop()

	// 完成 initialize 握手
	if err := c.initialize(ctx, cfg); err != nil {
		_ = transport.Close()
		return fmt.Errorf("initialize %q: %w", c.serverName, err)
	}
	return nil
}

// createTransport 根据配置创建对应的传输实现。
func createTransport(serverName string, cfg MCPServerConfig) (Transport, error) {
	switch cfg.Transport() {
	case "stdio":
		return newStdioTransport(serverName, cfg)
	case "sse":
		return newSSETransport(serverName, cfg)
	case "http":
		// Streamable HTTP 复用 SSE 传输（POST 请求 + 读取响应）。
		return newSSETransport(serverName, cfg)
	default:
		return nil, fmt.Errorf("server %q: no transport configured (need command/url/httpUrl)", serverName)
	}
}

// ---------------------------------------------------------------------------
// initialize 握手
// ---------------------------------------------------------------------------

func (c *Client) initialize(ctx context.Context, cfg MCPServerConfig) error {
	timeout := ConnectTimeout
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := InitializeParams{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities:    ClientCapabilities{},
		ClientInfo: ClientInfo{
			Name:    MCPClientName,
			Version: MCPClientVersion,
		},
	}

	result, err := c.request(ctx, "initialize", params)
	if err != nil {
		return err
	}

	var initResult InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}

	c.serverInfo = initResult.ServerInfo
	c.serverCaps = initResult.Capabilities
	c.initialized = true

	// 发送 initialized 通知（MCP 协议要求）
	if err := c.notify(ctx, "notifications/initialized", nil); err != nil {
		slog.Debug("MCP initialized notification failed", "server", c.serverName, "err", err)
	}

	slog.Info("MCP client initialized",
		"server", c.serverName,
		"serverName", initResult.ServerInfo.Name,
		"serverVersion", initResult.ServerInfo.Version,
		"protocol", initResult.ProtocolVersion)
	return nil
}

// ---------------------------------------------------------------------------
// JSON-RPC 请求 / 通知
// ---------------------------------------------------------------------------

// request 发送一个 JSON-RPC 请求并等待响应结果（result 字段）。
func (c *Client) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1)

	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		paramsRaw = b
	}

	// 注册 pending channel（在发送前注册，避免竞态）
	ch := make(chan *rpcResult, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, c.closeErr
	}
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      mustMarshalJSON(id),
		Method:  method,
		Params:  paramsRaw,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	slog.Debug("MCP → sending request", "server", c.serverName, "method", method, "id", id)
	if err := c.transport.Send(ctx, data); err != nil {
		return nil, fmt.Errorf("send %s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.parse != nil {
			return nil, fmt.Errorf("parse response: %w", r.parse)
		}
		if r.err != nil {
			return nil, fmt.Errorf("rpc error %d: %s", r.err.Error.Code, r.err.Error.Message)
		}
		return r.resp.Result, nil
	}
}

// notify 发送一个不带 id 的 JSON-RPC 通知（不等待响应）。
func (c *Client) notify(ctx context.Context, method string, params any) error {
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		paramsRaw = b
	}
	notif := JSONRPCNotification{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  paramsRaw,
	}
	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	return c.transport.Send(ctx, data)
}

// readLoop 在独立 goroutine 中持续读取传输层数据，按 id 分发响应。
func (c *Client) readLoop() {
	for {
		data, err := c.transport.Recv(context.Background())
		if err != nil {
			c.failAll(err)
			return
		}

		// 尝试解析为响应（可能是 response 或 error response）
		var resp JSONRPCResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			slog.Debug("MCP ← unparseable message", "server", c.serverName, "data", string(data))
			continue
		}

		// 检查是否是 error response
		var errResp JSONRPCErrorResponse
		if json.Unmarshal(data, &errResp) == nil && errResp.Error.Code != 0 {
			id, _ := parseID(errResp.ID)
			c.dispatch(id, &rpcResult{err: &errResp})
			continue
		}

		// 有 id 的成功响应
		if len(resp.ID) > 0 {
			id, _ := parseID(resp.ID)
			c.dispatch(id, &rpcResult{resp: &resp})
			continue
		}

		// 无 id → 通知，忽略（当前实现不处理服务器通知）
		slog.Debug("MCP ← notification (ignored)", "server", c.serverName, "data", string(data))
	}
}

// dispatch 将响应投递给等待该 id 的 channel。
func (c *Client) dispatch(id int64, r *rpcResult) {
	c.mu.Lock()
	ch, ok := c.pending[id]
	c.mu.Unlock()
	if ok {
		ch <- r
	}
}

// failAll 在传输层断开时，唤醒所有等待中的请求并返回错误。
func (c *Client) failAll(err error) {
	c.mu.Lock()
	c.closed = true
	c.closeErr = err
	pending := c.pending
	c.pending = make(map[int64]chan *rpcResult)
	c.mu.Unlock()

	for _, ch := range pending {
		ch <- &rpcResult{parse: err}
	}
}

// Close 关闭客户端连接。
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return c.closeErr
	}
	c.closed = true
	c.mu.Unlock()

	if c.transport != nil {
		return c.transport.Close()
	}
	return nil
}

// ---------------------------------------------------------------------------
// 高层 API：tools / resources / prompts
// ---------------------------------------------------------------------------

// ListTools 调用 tools/list，返回服务器提供的工具列表。
func (c *Client) ListTools(ctx context.Context) ([]MCPTool, error) {
	if c.serverCaps.Tools == nil {
		return nil, nil // 服务器不支持工具
	}
	result, err := c.request(ctx, "tools/list", struct{}{})
	if err != nil {
		return nil, err
	}
	var lr ListToolsResult
	if err := json.Unmarshal(result, &lr); err != nil {
		return nil, fmt.Errorf("parse tools/list result: %w", err)
	}
	return lr.Tools, nil
}

// CallTool 调用 tools/call，执行指定工具。
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (*CallToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: arguments,
	}
	result, err := c.request(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}
	var cr CallToolResult
	if err := json.Unmarshal(result, &cr); err != nil {
		return nil, fmt.Errorf("parse tools/call result: %w", err)
	}
	return &cr, nil
}

// ListResources 调用 resources/list。
func (c *Client) ListResources(ctx context.Context) ([]MCPResource, error) {
	if c.serverCaps.Resources == nil {
		return nil, nil
	}
	result, err := c.request(ctx, "resources/list", struct{}{})
	if err != nil {
		return nil, err
	}
	var lr ListResourcesResult
	if err := json.Unmarshal(result, &lr); err != nil {
		return nil, fmt.Errorf("parse resources/list result: %w", err)
	}
	return lr.Resources, nil
}

// ReadResource 调用 resources/read。
func (c *Client) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	result, err := c.request(ctx, "resources/read", map[string]string{"uri": uri})
	if err != nil {
		return nil, err
	}
	var rr ReadResourceResult
	if err := json.Unmarshal(result, &rr); err != nil {
		return nil, fmt.Errorf("parse resources/read result: %w", err)
	}
	return &rr, nil
}

// ListPrompts 调用 prompts/list。
func (c *Client) ListPrompts(ctx context.Context) ([]MCPPrompt, error) {
	if c.serverCaps.Prompts == nil {
		return nil, nil
	}
	result, err := c.request(ctx, "prompts/list", struct{}{})
	if err != nil {
		return nil, err
	}
	var lr ListPromptsResult
	if err := json.Unmarshal(result, &lr); err != nil {
		return nil, fmt.Errorf("parse prompts/list result: %w", err)
	}
	return lr.Prompts, nil
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

func mustMarshalJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// parseID 将 JSON-RPC id（可能是 number 或 string）解析为 int64。
// 字符串 id 会被哈希——实际上 MCP 客户端总是发送数字 id。
func parseID(raw json.RawMessage) (int64, error) {
	// 尝试数字
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	// 尝试字符串（罕见情况，直接返回错误——我们的 id 总是数字）
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return 0, fmt.Errorf("string id %q not supported", s)
	}
	return 0, errors.New("invalid id")
}

// ===========================================================================
// stdio 传输
// ===========================================================================

// stdioTransport 通过子进程的 stdin/stdout 管道传输 JSON-RPC。
type stdioTransport struct {
	serverName string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	scanner    *bufio.Scanner
}

func newStdioTransport(serverName string, cfg MCPServerConfig) (*stdioTransport, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("server %q: stdio transport requires 'command'", serverName)
	}

	cmd := exec.Command(cfg.Command, cfg.Args...)
	if cfg.Cwd != "" {
		cmd.Dir = cfg.Cwd
	}
	// 合并环境变量
	if len(cfg.Env) > 0 {
		cmd.Env = mergeEnv(cfg.Env)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("server %q: create stdin pipe: %w", serverName, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("server %q: create stdout pipe: %w", serverName, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("server %q: create stderr pipe: %w", serverName, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("server %q: start command %q: %w", serverName, cfg.Command, err)
	}

	t := &stdioTransport{
		serverName: serverName,
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		scanner:    bufio.NewScanner(stdout),
	}
	// MCP 消息可能较大（如 resources/read 返回大文件），增大 buffer
	t.scanner.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)

	// 后台读取 stderr 用于调试
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			slog.Debug("MCP stderr", "server", serverName, "line", sc.Text())
		}
	}()

	slog.Info("MCP stdio transport started", "server", serverName, "command", cfg.Command)
	return t, nil
}

func (t *stdioTransport) Send(_ context.Context, data []byte) error {
	// JSON-RPC over stdio：每条消息占一行
	if _, err := t.stdin.Write(data); err != nil {
		return err
	}
	_, err := t.stdin.Write([]byte("\n"))
	return err
}

func (t *stdioTransport) Recv(_ context.Context) ([]byte, error) {
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	return t.scanner.Bytes(), nil
}

func (t *stdioTransport) Close() error {
	// 先关闭 stdin，让子进程退出
	_ = t.stdin.Close()
	// 等待进程退出
	if t.cmd.Process != nil {
		done := make(chan error, 1)
		go func() { done <- t.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = t.cmd.Process.Kill()
			<-done
		}
	}
	return nil
}

// mergeEnv 将额外环境变量合并到 os.Environ()。
func mergeEnv(extra map[string]string) []string {
	// cmd.Env = nil 时子进程继承父进程环境（默认行为）。
	// 只有显式设置 Env 时才需要完整列表，这里合并两者。
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

// ===========================================================================
// SSE 传输
// ===========================================================================

// sseTransport 通过 HTTP POST 发送请求，通过 SSE 接收响应。
//
// 当前实现使用简化模型：每次请求通过 HTTP POST 发送，
// 服务器在响应体中直接返回 JSON-RPC response（兼容简单 HTTP 模式）。
// 完整的 SSE 模式（长连接 + event-id 关联）留作后续扩展。
type sseTransport struct {
	serverName string
	url        string
	headers    map[string]string
	httpClient *http.Client
	respCh     chan []byte
}

func newSSETransport(serverName string, cfg MCPServerConfig) (*sseTransport, error) {
	url := cfg.HTTPURL
	if url == "" {
		url = cfg.URL
	}
	if url == "" {
		return nil, fmt.Errorf("server %q: SSE/HTTP transport requires 'url' or 'httpUrl'", serverName)
	}

	return &sseTransport{
		serverName: serverName,
		url:        url,
		headers:    cfg.Headers,
		httpClient: &http.Client{Timeout: DefaultTimeout},
		respCh:     make(chan []byte, 16),
	}, nil
}

func (t *sseTransport) Send(ctx context.Context, data []byte) error {
	// 直接通过 HTTP POST 发送，响应存入内部缓冲区
	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}

	// 将响应体放入管道，供 Recv 读取
	go func() {
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			// SSE 格式：data: {...}
			if bytes.HasPrefix(line, []byte("data:")) {
				payload := bytes.TrimSpace(line[5:])
				select {
				case t.respCh <- append([]byte(nil), payload...):
				case <-ctx.Done():
					return
				}
			} else if len(bytes.TrimSpace(line)) > 0 && line[0] == '{' {
				// 直接 JSON 响应（非 SSE 格式）
				select {
				case t.respCh <- append([]byte(nil), line...):
				case <-ctx.Done():
					return
				}
			}
		}
		close(t.respCh)
	}()

	return nil
}

func (t *sseTransport) Recv(_ context.Context) ([]byte, error) {
	data, ok := <-t.respCh
	if !ok {
		return nil, io.EOF
	}
	return data, nil
}

func (t *sseTransport) Close() error {
	return nil
}


