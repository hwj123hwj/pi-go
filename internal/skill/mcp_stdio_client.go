package skill

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultMCPProtocolVersion = "2024-11-05"
	defaultMCPStdioTimeout    = 20 * time.Second
	defaultMCPMaxMessageBytes = 8 << 20
)

// StdioMCPSkillClient talks to an MCP server over stdio JSON-RPC framing and
// exposes the resource list/read subset needed by MCPSkillSourceProvider.
type StdioMCPSkillClient struct {
	Command         string
	Args            []string
	Env             []string
	ProtocolVersion string
	Timeout         time.Duration
	MaxMessageBytes int64
	requestID       atomic.Int64
}

func NewStdioMCPSkillClient(command string, args ...string) *StdioMCPSkillClient {
	return &StdioMCPSkillClient{
		Command:         strings.TrimSpace(command),
		Args:            append([]string(nil), args...),
		ProtocolVersion: defaultMCPProtocolVersion,
		Timeout:         defaultMCPStdioTimeout,
		MaxMessageBytes: defaultMCPMaxMessageBytes,
	}
}

func (c *StdioMCPSkillClient) ListSkillResources(ctx context.Context) ([]MCPSkillResource, error) {
	var result mcpResourcesListResult
	if err := c.call(ctx, "resources/list", map[string]any{}, &result); err != nil {
		return nil, err
	}
	out := make([]MCPSkillResource, 0, len(result.Resources))
	for _, res := range result.Resources {
		uri := strings.TrimSpace(res.URI)
		if uri == "" {
			continue
		}
		out = append(out, MCPSkillResource{URI: uri, Name: strings.TrimSpace(res.Name)})
	}
	return out, nil
}

func (c *StdioMCPSkillClient) ReadSkillResource(ctx context.Context, uri string) ([]byte, error) {
	var result mcpResourcesReadResult
	if err := c.call(ctx, "resources/read", map[string]any{"uri": uri}, &result); err != nil {
		return nil, err
	}
	return mcpResourceContents(uri, result)
}

func mcpResourceContents(uri string, result mcpResourcesReadResult) ([]byte, error) {
	var parts [][]byte
	for _, content := range result.Contents {
		if content.Text != "" {
			parts = append(parts, []byte(content.Text))
			continue
		}
		if content.Blob != "" {
			data, err := base64.StdEncoding.DecodeString(content.Blob)
			if err != nil {
				return nil, fmt.Errorf("decode mcp resource blob %q: %w", uri, err)
			}
			parts = append(parts, data)
		}
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("mcp resource %q returned no text or blob contents", uri)
	}
	return bytes.Join(parts, []byte("\n")), nil
}

func (c *StdioMCPSkillClient) call(ctx context.Context, method string, params any, result any) error {
	command := strings.TrimSpace(c.Command)
	if command == "" {
		return fmt.Errorf("mcp stdio command is empty")
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultMCPStdioTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(callCtx, command, c.Args...)
	if len(c.Env) > 0 {
		cmd.Env = append(cmd.Environ(), c.Env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("open mcp stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open mcp stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start mcp stdio server: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)
	if err := c.initialize(callCtx, stdin, reader); err != nil {
		return appendMCPStderr(err, stderr.String())
	}
	if err := c.rpc(callCtx, stdin, reader, method, params, result); err != nil {
		return appendMCPStderr(err, stderr.String())
	}
	return nil
}

func (c *StdioMCPSkillClient) initialize(ctx context.Context, writer io.Writer, reader *bufio.Reader) error {
	protocolVersion := strings.TrimSpace(c.ProtocolVersion)
	if protocolVersion == "" {
		protocolVersion = defaultMCPProtocolVersion
	}
	var ignored json.RawMessage
	if err := c.rpc(ctx, writer, reader, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]string{
			"name":    "pi-go",
			"version": "0",
		},
	}, &ignored); err != nil {
		return err
	}
	return writeMCPMessage(writer, mcpRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]any{},
	})
}

func (c *StdioMCPSkillClient) rpc(ctx context.Context, writer io.Writer, reader *bufio.Reader, method string, params any, result any) error {
	id := c.requestID.Add(1)
	if err := writeMCPMessage(writer, mcpRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}); err != nil {
		return fmt.Errorf("write mcp request %q: %w", method, err)
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := readMCPMessage(reader, c.maxMessageBytes())
		if err != nil {
			return fmt.Errorf("read mcp response %q: %w", method, err)
		}
		var resp mcpResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("parse mcp response %q: %w", method, err)
		}
		if resp.ID == nil {
			continue
		}
		if !mcpIDEqual(resp.ID, id) {
			continue
		}
		if resp.Error != nil {
			return fmt.Errorf("mcp request %q failed: %s", method, resp.Error.Message)
		}
		if result == nil || len(resp.Result) == 0 {
			return nil
		}
		if raw, ok := result.(*json.RawMessage); ok {
			*raw = append((*raw)[:0], resp.Result...)
			return nil
		}
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("parse mcp result %q: %w", method, err)
		}
		return nil
	}
}

func (c *StdioMCPSkillClient) maxMessageBytes() int64 {
	if c.MaxMessageBytes > 0 {
		return c.MaxMessageBytes
	}
	return defaultMCPMaxMessageBytes
}

func writeMCPMessage(writer io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func readMCPMessage(reader *bufio.Reader, maxBytes int64) ([]byte, error) {
	var contentLength int64
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid content-length %q", strings.TrimSpace(value))
			}
			contentLength = n
		}
	}
	if contentLength == 0 {
		return nil, fmt.Errorf("missing content-length")
	}
	if maxBytes > 0 && contentLength > maxBytes {
		return nil, fmt.Errorf("mcp message exceeds %d bytes", maxBytes)
	}
	data := make([]byte, contentLength)
	_, err := io.ReadFull(reader, data)
	return data, err
}

func mcpIDEqual(raw json.RawMessage, id int64) bool {
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n == id
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int64(f) == id
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s == strconv.FormatInt(id, 10)
	}
	return false
}

func appendMCPStderr(err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return err
	}
	if len(stderr) > 2048 {
		stderr = stderr[:2048] + "..."
	}
	return fmt.Errorf("%w; stderr: %s", err, stderr)
}

type mcpRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpResourcesListResult struct {
	Resources []struct {
		URI  string `json:"uri"`
		Name string `json:"name"`
	} `json:"resources"`
}

type mcpResourcesReadResult struct {
	Contents []struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
		Blob string `json:"blob"`
	} `json:"contents"`
}
