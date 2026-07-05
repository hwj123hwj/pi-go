package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"os/exec"
)

// ─── LSP Manager ──────────────────────────────────────────────────────────────
//
// Manager owns the lifecycle of language server processes and provides
// high-level operations (hover, definition, references, ...).
//
// It is safe for concurrent use: multiple tools can call operations in parallel.
// Each (serverID, rootDir) pair maps to a single long-lived Client.

// Client wraps a running language server process and its JSON-RPC connection.
type Client struct {
	ServerID string
	Root     string
	cmd      *exec.Cmd
	conn     *Conn

	// Track files opened with this client for didOpen/didChange bookkeeping
	docMu       sync.Mutex
	openedFiles map[string]*docState

	// io.Closer helpers for cleanup
	stdinCloser  io.WriteCloser
	stdoutCloser io.ReadCloser
}

type docState struct {
	version int
	content string
}

// Manager manages multiple language server clients.
type Manager struct {
	projectRoot string
	servers     []ServerSpec

	mu      sync.Mutex
	clients map[string]*Client // key = serverID + ":" + root
}

// NewManager creates a new LSP Manager rooted at projectRoot.
func NewManager(projectRoot string) *Manager {
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		abs = projectRoot
	}
	return &Manager{
		projectRoot: abs,
		servers:     DefaultServers(),
		clients:     make(map[string]*Client),
	}
}

// SetServers overrides the default server list (for testing or extensibility).
func (m *Manager) SetServers(servers []ServerSpec) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers = servers
}

// ProjectRoot returns the root directory the manager was created with.
func (m *Manager) ProjectRoot() string {
	return m.projectRoot
}

// ─── Client lifecycle ─────────────────────────────────────────────────────────

// getClientsForFile returns all running clients that handle the file's extension,
// starting new server processes as needed.
func (m *Manager) getClientsForFile(ctx context.Context, file string) ([]*Client, error) {
	absFile, err := filepath.Abs(file)
	if err != nil {
		absFile = file
	}

	matching := findServersForFile(m.servers, absFile)
	if len(matching) == 0 {
		return nil, fmt.Errorf("no language server configured for file type %s", filepath.Ext(absFile))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var clients []*Client
	for _, spec := range matching {
		root := spec.RootDir(absFile, m.projectRoot)
		key := spec.ID + ":" + root

		if existing, ok := m.clients[key]; ok {
			clients = append(clients, existing)
			continue
		}

		client, err := m.startClient(ctx, spec, root)
		if err != nil {
			slog.Warn("LSP failed to start client", "server", spec.ID, "root", root, "err", err)
			continue // skip failed servers, don't abort all
		}
		m.clients[key] = client
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("failed to start any language server for %s", absFile)
	}
	return clients, nil
}

// startClient spawns the server process and performs the LSP initialize handshake.
func (m *Manager) startClient(ctx context.Context, spec ServerSpec, root string) (*Client, error) {
	cmd, err := spec.Spawn(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("spawn %s: %w", spec.ID, err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	cmd.Stderr = &logWriter{serverID: spec.ID}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("start process: %w", err)
	}

	slog.Info("LSP server started", "server", spec.ID, "root", root, "pid", cmd.Process.Pid)

	conn := NewConn(stdout, stdin)
	registerDefaultHandlers(conn, root)

	// Start read loop in background
	go conn.Start()

	// Watch for process exit
	go func() {
		err := cmd.Wait()
		conn.Close()
		if err != nil {
			slog.Warn("LSP server exited with error", "server", spec.ID, "err", err)
		} else {
			slog.Debug("LSP server exited cleanly", "server", spec.ID)
		}
	}()

	client := &Client{
		ServerID:     spec.ID,
		Root:         root,
		cmd:          cmd,
		conn:         conn,
		openedFiles:  make(map[string]*docState),
		stdinCloser:  stdin,
		stdoutCloser: stdout,
	}

	// LSP initialize handshake
	if err := client.initialize(ctx); err != nil {
		client.kill()
		return nil, fmt.Errorf("initialize %s: %w", spec.ID, err)
	}

	return client, nil
}

// initialize performs the LSP initialize / initialized handshake.
func (c *Client) initialize(ctx context.Context) error {
	rootURI := PathToURI(c.Root)

	params := map[string]any{
		"processId": os.Getpid(),
		"rootPath":  c.Root,
		"rootUri":   rootURI,
		"capabilities": map[string]any{
			"window": map[string]any{
				"workDoneProgress": true,
			},
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"dynamicRegistration": true,
					"willSave":            false,
					"willSaveWaitUntil":   false,
					"didSave":             true,
					"didChange":           1, // 1 = Full sync
				},
				"hover": map[string]any{
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"definition": map[string]any{
					"dynamicRegistration": true,
					"linkSupport":         true,
				},
				"references": map[string]any{
					"dynamicRegistration": true,
				},
				"documentSymbol": map[string]any{
					"dynamicRegistration":            true,
					"hierarchicalDocumentSymbolSupport": true,
				},
				"implementation": map[string]any{
					"dynamicRegistration": true,
					"linkSupport":         true,
				},
			},
			"workspace": map[string]any{
				"workspaceFolders": true,
				"configuration":    true,
				"symbol": map[string]any{
					"dynamicRegistration": true,
				},
			},
		},
		"workspaceFolders": []map[string]any{
			{"uri": rootURI, "name": filepath.Base(c.Root)},
		},
	}

	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := c.conn.Call(initCtx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize request: %w", err)
	}

	// We don't deeply parse the server capabilities; gopls supports all
	// operations we use. Storing for future use would go here.
	_ = result

	if err := c.conn.Notify("initialized", map[string]any{}); err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}

	// Send an empty configuration change to nudge servers that wait for it
	_ = c.conn.Notify("workspace/didChangeConfiguration", map[string]any{
		"settings": map[string]any{},
	})

	return nil
}

// ─── Document synchronization ─────────────────────────────────────────────────

// syncDocument sends didOpen (first time) or didChange (content updated).
func (m *Manager) syncDocument(ctx context.Context, client *Client, file string) error {
	absFile, err := filepath.Abs(file)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("read file for sync: %w", err)
	}
	text := string(content)
	uri := PathToURI(absFile)

	client.docMu.Lock()
	doc, exists := client.openedFiles[uri]
	client.docMu.Unlock()

	if !exists {
		// didOpen
		if err := client.conn.Notify("textDocument/didOpen", map[string]any{
			"textDocument": map[string]any{
				"uri":        uri,
				"languageId": languageID(absFile),
				"version":    1,
				"text":       text,
			},
		}); err != nil {
			return fmt.Errorf("didOpen: %w", err)
		}
		client.docMu.Lock()
		client.openedFiles[uri] = &docState{version: 1, content: text}
		client.docMu.Unlock()

		// Give the server a moment to parse the newly opened document.
		// This is especially important for gopls on first open.
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	}

	// didChange (only if content changed)
	if doc.content == text {
		return nil
	}

	doc.version++
	doc.content = text

	return client.conn.Notify("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{
			"uri":     uri,
			"version": doc.version,
		},
		"contentChanges": []map[string]any{
			{"text": text},
		},
	})
}

// ─── run: the generic operation wrapper ──────────────────────────────────────
//
// Mirrors hwjcode's Manager.run: get clients, sync document, execute task,
// collect results. One key difference: we return aggregated results as
// json.RawMessage slices, leaving JSON type interpretation to the caller.

// run executes a task against all clients matching the file, collecting results.
func (m *Manager) run(ctx context.Context, file string, timeout time.Duration, task func(ctx context.Context, c *Client) (json.RawMessage, error)) ([]json.RawMessage, error) {
	clients, err := m.getClientsForFile(ctx, file)
	if err != nil {
		return nil, err
	}

	var results []json.RawMessage
	for _, client := range clients {
		if err := m.syncDocument(ctx, client, file); err != nil {
			slog.Warn("LSP document sync failed", "server", client.ServerID, "file", file, "err", err)
			continue
		}

		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		result, err := task(reqCtx, client)
		cancel()

		if err != nil {
			slog.Warn("LSP request failed", "server", client.ServerID, "err", err)
			continue
		}
		if len(result) > 0 && string(result) != "null" {
			results = append(results, result)
		}
	}
	return results, nil
}

// ─── High-level LSP operations ────────────────────────────────────────────────
//
// Each operation returns the raw JSON result(s) from the server.
// The tools layer (internal/tools/lsp.go) interprets and formats these.

// GetHover sends textDocument/hover.
func (m *Manager) GetHover(ctx context.Context, file string, line, character int) ([]json.RawMessage, error) {
	return m.run(ctx, file, 15*time.Second, func(ctx context.Context, c *Client) (json.RawMessage, error) {
		return c.conn.Call(ctx, "textDocument/hover", map[string]any{
			"textDocument": map[string]any{"uri": PathToURI(file)},
			"position":     Position{Line: line, Character: character},
		})
	})
}

// GetDefinition sends textDocument/definition.
func (m *Manager) GetDefinition(ctx context.Context, file string, line, character int) ([]json.RawMessage, error) {
	return m.run(ctx, file, 15*time.Second, func(ctx context.Context, c *Client) (json.RawMessage, error) {
		return c.conn.Call(ctx, "textDocument/definition", map[string]any{
			"textDocument": map[string]any{"uri": PathToURI(file)},
			"position":     Position{Line: line, Character: character},
		})
	})
}

// GetReferences sends textDocument/references.
func (m *Manager) GetReferences(ctx context.Context, file string, line, character int) ([]json.RawMessage, error) {
	return m.run(ctx, file, 15*time.Second, func(ctx context.Context, c *Client) (json.RawMessage, error) {
		result, err := c.conn.Call(ctx, "textDocument/references", map[string]any{
			"textDocument": map[string]any{"uri": PathToURI(file)},
			"position":     Position{Line: line, Character: character},
			"context":      map[string]any{"includeDeclaration": true},
		})
		if err != nil {
			return nil, err
		}
		// Retry once if empty (server may still be indexing)
		if isEmptyArray(result) {
			slog.Debug("LSP references empty, retrying", "server", c.ServerID)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			result, err = c.conn.Call(ctx, "textDocument/references", map[string]any{
				"textDocument": map[string]any{"uri": PathToURI(file)},
				"position":     Position{Line: line, Character: character},
				"context":      map[string]any{"includeDeclaration": true},
			})
		}
		return result, err
	})
}

// GetImplementation sends textDocument/implementation.
func (m *Manager) GetImplementation(ctx context.Context, file string, line, character int) ([]json.RawMessage, error) {
	return m.run(ctx, file, 15*time.Second, func(ctx context.Context, c *Client) (json.RawMessage, error) {
		result, err := c.conn.Call(ctx, "textDocument/implementation", map[string]any{
			"textDocument": map[string]any{"uri": PathToURI(file)},
			"position":     Position{Line: line, Character: character},
		})
		if err != nil {
			return nil, err
		}
		if isEmptyArray(result) {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			result, err = c.conn.Call(ctx, "textDocument/implementation", map[string]any{
				"textDocument": map[string]any{"uri": PathToURI(file)},
				"position":     Position{Line: line, Character: character},
			})
		}
		return result, err
	})
}

// GetDocumentSymbols sends textDocument/documentSymbol.
func (m *Manager) GetDocumentSymbols(ctx context.Context, file string) ([]json.RawMessage, error) {
	return m.run(ctx, file, 15*time.Second, func(ctx context.Context, c *Client) (json.RawMessage, error) {
		return c.conn.Call(ctx, "textDocument/documentSymbol", map[string]any{
			"textDocument": map[string]any{"uri": PathToURI(file)},
		})
	})
}

// GetWorkspaceSymbols sends workspace/symbol.
func (m *Manager) GetWorkspaceSymbols(ctx context.Context, query string) ([]json.RawMessage, error) {
	// Ensure at least one client is running. We probe the project root for
	// source files to activate the appropriate server.
	m.mu.Lock()
	hasClients := len(m.clients) > 0
	m.mu.Unlock()

	if !hasClients {
		if err := m.activateForProject(ctx); err != nil {
			slog.Warn("LSP failed to activate any server for workspace symbols", "err", err)
		}
	}

	m.mu.Lock()
	allClients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		allClients = append(allClients, c)
	}
	m.mu.Unlock()

	var results []json.RawMessage
	for _, client := range allClients {
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		result, err := client.conn.Call(reqCtx, "workspace/symbol", map[string]any{
			"query": query,
		})
		cancel()
		if err != nil {
			slog.Warn("LSP workspace/symbol failed", "server", client.ServerID, "err", err)
			continue
		}
		if len(result) > 0 && string(result) != "null" {
			results = append(results, result)
		}
	}
	return results, nil
}

// activateForProject probes the project root for source files and starts
// the matching language server.
func (m *Manager) activateForProject(ctx context.Context) error {
	// Walk the project root (limited depth) to find a source file we can use
	// to activate a server.
	probeExts := []string{".go", ".ts", ".py", ".rs", ".js"}
	found := findFirstFile(m.projectRoot, probeExts, 3)
	if found == "" {
		return fmt.Errorf("no source files found in project root")
	}

	clients, err := m.getClientsForFile(ctx, found)
	if err != nil {
		return err
	}
	for _, c := range clients {
		if err := m.syncDocument(ctx, c, found); err != nil {
			slog.Warn("LSP sync during activation failed", "server", c.ServerID, "err", err)
		}
	}
	// Give server time to index
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
	}
	return nil
}

// ─── Shutdown ─────────────────────────────────────────────────────────────────

// Shutdown stops all language server processes gracefully.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, client := range m.clients {
		m.shutdownClient(ctx, client)
		delete(m.clients, key)
	}
}

func (m *Manager) shutdownClient(ctx context.Context, client *Client) {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Send shutdown request
	if _, err := client.conn.Call(shutdownCtx, "shutdown", nil); err != nil {
		slog.Warn("LSP shutdown request failed", "server", client.ServerID, "err", err)
	}
	// Send exit notification
	_ = client.conn.Notify("exit", nil)

	client.conn.Close()
	client.kill()
}

func (c *Client) kill() {
	if c.stdinCloser != nil {
		c.stdinCloser.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// isEmptyArray returns true if raw is "null" or "[]" or empty.
func isEmptyArray(raw json.RawMessage) bool {
	s := string(raw)
	return s == "" || s == "null" || s == "[]"
}

// findFirstFile walks dir up to maxDepth levels and returns the first file
// whose extension matches one of exts.
func findFirstFile(dir string, exts []string, maxDepth int) string {
	var result string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "vendor" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := filepath.Ext(path)
		for _, e := range exts {
			if ext == e {
				result = path
				return errStopWalk
			}
		}
		return nil
	})
	return result
}

// errStopWalk is a sentinel error to stop filepath.WalkDir early.
var errStopWalk = fmt.Errorf("stop walk")

// ─── Default server→client request handlers ───────────────────────────────────

// registerDefaultHandlers installs handlers for common server→client requests
// so the server doesn't block waiting for responses we don't care about.
func registerDefaultHandlers(conn *Conn, root string) {
	// workspace/configuration — return empty config for each requested item
	conn.RegisterHandler("workspace/configuration", func(params json.RawMessage) (any, error) {
		var p struct {
			Items []map[string]any `json:"items"`
		}
		_ = json.Unmarshal(params, &p)
		results := make([]map[string]any, len(p.Items))
		for i := range results {
			results[i] = map[string]any{}
		}
		return results, nil
	})

	// client/registerCapability
	conn.RegisterHandler("client/registerCapability", func(params json.RawMessage) (any, error) {
		return map[string]any{}, nil
	})

	// client/unregisterCapability
	conn.RegisterHandler("client/unregisterCapability", func(params json.RawMessage) (any, error) {
		return map[string]any{}, nil
	})

	// window/workDoneProgress/create
	conn.RegisterHandler("window/workDoneProgress/create", func(params json.RawMessage) (any, error) {
		return nil, nil
	})

	// workspace/semanticTokens/refresh
	conn.RegisterHandler("workspace/semanticTokens/refresh", func(params json.RawMessage) (any, error) {
		return nil, nil
	})

	// workspace/inlayHint/refresh
	conn.RegisterHandler("workspace/inlayHint/refresh", func(params json.RawMessage) (any, error) {
		return nil, nil
	})

	// workspace/codeLens/refresh
	conn.RegisterHandler("workspace/codeLens/refresh", func(params json.RawMessage) (any, error) {
		return nil, nil
	})

	// workspace/diagnostic/refresh
	conn.RegisterHandler("workspace/diagnostic/refresh", func(params json.RawMessage) (any, error) {
		return nil, nil
	})

	// window/showMessageRequest
	conn.RegisterHandler("window/showMessageRequest", func(params json.RawMessage) (any, error) {
		var p struct {
			Actions []map[string]any `json:"actions"`
		}
		_ = json.Unmarshal(params, &p)
		if len(p.Actions) > 0 {
			return p.Actions[0], nil
		}
		return nil, nil
	})

	// workspace/applyEdit
	conn.RegisterHandler("workspace/applyEdit", func(params json.RawMessage) (any, error) {
		return map[string]any{"applied": true}, nil
	})

	// workspace/workspaceFolders
	conn.RegisterHandler("workspace/workspaceFolders", func(params json.RawMessage) (any, error) {
		return []map[string]any{
			{"uri": PathToURI(root), "name": filepath.Base(root)},
		}, nil
	})
}

// ─── logWriter (server stderr → slog) ─────────────────────────────────────────

// logWriter pipes server process stderr to slog at Debug level.
type logWriter struct {
	serverID string
}

func (lw *logWriter) Write(p []byte) (int, error) {
	msg := string(p)
	// Trim trailing whitespace
	for len(msg) > 0 && (msg[len(msg)-1] == '\n' || msg[len(msg)-1] == '\r') {
		msg = msg[:len(msg)-1]
	}
	if msg != "" {
		slog.Debug("LSP server stderr", "server", lw.serverID, "msg", msg)
	}
	return len(p), nil
}
