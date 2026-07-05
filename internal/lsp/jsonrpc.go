package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// ─── JSON-RPC 2.0 over stdio ──────────────────────────────────────────────────
//
// LSP communicates via JSON-RPC 2.0 messages framed with Content-Length headers.
// This file implements the framing layer (read/write) and a minimal request/
// response correlation layer.
//
// Message format (LF line endings):
//
//	Content-Length: <N>\r\n
//	\r\n
//	<N bytes of JSON>
//
// Three message types: Request, Response, Notification.

// rpcRequest is the JSON-RPC 2.0 request envelope.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse is the JSON-RPC 2.0 response envelope.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// rpcNotification is a JSON-RPC 2.0 notification (no ID, no response expected).
type rpcNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcMessage is used for decoding — it captures all fields regardless of type.
type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

// ─── Conn: bidirectional JSON-RPC connection ─────────────────────────────────

// Conn wraps an io.ReadWriter (typically a process's stdin+stdout) into a
// JSON-RPC 2.0 connection that supports asynchronous requests, notifications,
// and server-to-client request handlers.
type Conn struct {
	r        io.Reader
	w        io.Writer
	writeMu  sync.Mutex
	nextID   int64
	pending  sync.Map // map[string]chan *rpcResponse  (key = request ID)
	handlers map[string]serverRequestHandler
	handlerMu sync.RWMutex
	closed   atomic.Bool
	done     chan struct{}
}

// serverRequestHandler handles a server→client request.
// Return (result, error). The result will be marshalled and sent as the
// JSON-RPC response result field.
type serverRequestHandler func(params json.RawMessage) (any, error)

// NewConn creates a new JSON-RPC connection from separate reader and writer.
func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{
		r:        r,
		w:        w,
		nextID:   1,
		handlers: make(map[string]serverRequestHandler),
		done:     make(chan struct{}),
	}
}

// RegisterHandler sets a handler for a server→client request method.
// If handler is nil, the method is removed.
func (c *Conn) RegisterHandler(method string, handler serverRequestHandler) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	if handler == nil {
		delete(c.handlers, method)
	} else {
		c.handlers[method] = handler
	}
}

// Close marks the connection as closed. It does not close the underlying
// reader/writer — the caller (Manager) owns those via the process.
func (c *Conn) Close() {
	if c.closed.CompareAndSwap(false, true) {
		close(c.done)
	}
}

// Done returns a channel that is closed when the connection is closed.
func (c *Conn) Done() <-chan struct{} { return c.done }

// ─── Read loop ────────────────────────────────────────────────────────────────

// Start begins reading messages from the connection. It blocks until the
// underlying reader returns EOF/error or Close() is called.
// Incoming responses are dispatched to waiting callers via pending map.
// Incoming notifications/requests are handled inline.
func (c *Conn) Start() {
	br := bufio.NewReaderSize(c.r, 1<<20) // 1 MiB read buffer
	for !c.closed.Load() {
		msg, err := readMessage(br)
		if err != nil {
			if c.closed.Load() || err == io.EOF {
				slog.Debug("LSP conn read loop ended", "err", err)
			} else {
				slog.Warn("LSP conn read error", "err", err)
			}
			return
		}

		var m rpcMessage
		if err := json.Unmarshal(msg, &m); err != nil {
			slog.Warn("LSP conn failed to decode message", "err", err, "raw", truncateForLog(string(msg), 200))
			continue
		}

		// Dispatch based on message type
		if len(m.ID) > 0 && m.Method != "" {
			// Server → client request
			go c.handleServerRequest(m)
		} else if len(m.ID) > 0 {
			// Response to our request
			c.dispatchResponse(m)
		} else if m.Method != "" {
			// Notification (no ID) — we don't currently need to act on these
			slog.Debug("LSP notification received", "method", m.Method)
		}
	}
}

// handleServerRequest processes a server→client request and sends back a response.
func (c *Conn) handleServerRequest(m rpcMessage) {
	c.handlerMu.RLock()
	handler, ok := c.handlers[m.Method]
	c.handlerMu.RUnlock()

	var result any
	var rpcErr *rpcError

	if !ok {
		rpcErr = &rpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", m.Method)}
	} else {
		r, err := handler(m.Params)
		if err != nil {
			rpcErr = &rpcError{Code: -32603, Message: err.Error()}
		} else {
			result = r
		}
	}

	resp := rpcResponse{JSONRPC: "2.0", ID: m.ID}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		raw, _ := json.Marshal(result)
		resp.Result = raw
	}
	if err := c.sendMessage(resp); err != nil {
		slog.Warn("LSP failed to send server-request response", "method", m.Method, "err", err)
	}
}

// dispatchResponse delivers a response to the waiting caller.
func (c *Conn) dispatchResponse(m rpcMessage) {
	idKey := string(m.ID)
	ch, ok := c.pending.LoadAndDelete(idKey)
	if !ok {
		slog.Warn("LSP received response with unknown ID", "id", idKey)
		return
	}
	resp := &rpcResponse{
		JSONRPC: m.JSONRPC,
		ID:      m.ID,
		Result:  m.Result,
		Error:   m.Error,
	}
	ch.(chan *rpcResponse) <- resp
}

// ─── Sending requests / notifications ─────────────────────────────────────────

// Call sends a request and waits for the response (or context cancellation).
func (c *Conn) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c.closed.Load() {
		return nil, fmt.Errorf("connection closed")
	}

	id := atomic.AddInt64(&c.nextID, 1)
	idBytes, _ := json.Marshal(id)
	idStr := string(idBytes)

	respCh := make(chan *rpcResponse, 1)
	c.pending.Store(idStr, respCh)
	defer c.pending.Delete(idStr)

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      idBytes,
		Method:  method,
	}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params for %s: %w", method, err)
		}
		req.Params = raw
	}

	if err := c.sendMessage(req); err != nil {
		return nil, fmt.Errorf("send request %s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

// Notify sends a notification (no response expected).
func (c *Conn) Notify(method string, params any) error {
	if c.closed.Load() {
		return fmt.Errorf("connection closed")
	}
	notif := rpcNotification{
		JSONRPC: "2.0",
		Method:  method,
	}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params for %s: %w", method, err)
		}
		notif.Params = raw
	}
	return c.sendMessage(notif)
}

// sendMessage serialises and writes a single JSON-RPC message with framing.
func (c *Conn) sendMessage(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// Content-Length framing
	header := "Content-Length: " + strconv.Itoa(len(data)) + "\r\n\r\n"
	if _, err := io.WriteString(c.w, header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := c.w.Write(data); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	return nil
}

// ─── Message framing (read side) ──────────────────────────────────────────────

// readMessage reads a single Content-Length framed message from the reader.
func readMessage(br *bufio.Reader) ([]byte, error) {
	contentLength := 0
	// Read headers
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// Empty line signals end of headers
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(line[len("Content-Length:"):])
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q: %w", val, err)
			}
			contentLength = n
		}
		// Other headers (Content-Type, etc.) are ignored
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("missing or invalid Content-Length header")
	}

	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(br, buf); err != nil {
		return nil, fmt.Errorf("read body (%d bytes): %w", contentLength, err)
	}
	return buf, nil
}

// truncateForLog truncates a string for safe logging.
func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
