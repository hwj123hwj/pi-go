package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hwj123hwj/pi-go/internal/agent"
	"github.com/hwj123hwj/pi-go/internal/lsp"
)

// ─── LSP Tools ────────────────────────────────────────────────────────────────
//
// Six tools that expose Language Server Protocol operations to the agent:
//
//   - lsp_go_to_definition
//   - lsp_find_references
//   - lsp_hover
//   - lsp_document_symbols
//   - lsp_workspace_symbols
//   - lsp_go_to_implementation
//
// All tools share a per-workspace LSP Manager singleton (lazy initialised).
// LSP uses 0-based positions; all tools accept 1-based line/character from
// the LLM and convert internally.

// ─── Manager singleton ────────────────────────────────────────────────────────

var (
	globalLSPManager   *lsp.Manager
	globalLSPManagerMu sync.Mutex
)

// getLSPManager returns a process-wide singleton LSP Manager for the workspace.
func getLSPManager(workspace string) *lsp.Manager {
	globalLSPManagerMu.Lock()
	defer globalLSPManagerMu.Unlock()
	if globalLSPManager == nil {
		globalLSPManager = lsp.NewManager(workspace)
	}
	return globalLSPManager
}

// ResetLSPManager clears the global singleton (for testing).
func ResetLSPManager() {
	globalLSPManagerMu.Lock()
	defer globalLSPManagerMu.Unlock()
	if globalLSPManager != nil {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		globalLSPManager.Shutdown(ctx)
		globalLSPManager = nil
	}
}

// ─── Shared parameter structs ─────────────────────────────────────────────────

type lspPositionParams struct {
	FilePath  string `json:"filePath"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}

type lspFileParams struct {
	FilePath string `json:"filePath"`
}

type lspQueryParams struct {
	Query string `json:"query"`
}

// ─── 1. lsp_go_to_definition ──────────────────────────────────────────────────

// LSPGotoDefinitionTool finds the definition of the symbol at a position.
type LSPGotoDefinitionTool struct {
	workspace string
}

func NewLSPGotoDefinitionTool(workspace string) *LSPGotoDefinitionTool {
	return &LSPGotoDefinitionTool{workspace: workspace}
}

func (t *LSPGotoDefinitionTool) Name() string { return "lsp_go_to_definition" }

func (t *LSPGotoDefinitionTool) Description() string {
	return "Go to the definition of the symbol at a specific position in a file using Language Server Protocol. Useful for code navigation."
}

func (t *LSPGotoDefinitionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath":  map[string]any{"type": "string", "description": "The absolute path to the file to inspect."},
			"line":      map[string]any{"type": "integer", "description": "The 1-based line number (as shown in editors)."},
			"character": map[string]any{"type": "integer", "description": "The 1-based character offset on the line (as shown in editors)."},
		},
		"required": []string{"filePath", "line", "character"},
	}
}

func (t *LSPGotoDefinitionTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params lspPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if err := validatePositionParams(params); err != nil {
		return nil, err
	}
	return json.Marshal(params)
}

func (t *LSPGotoDefinitionTool) Execute(ctx context.Context, raw json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var params lspPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	mgr := getLSPManager(t.workspace)
	results, err := mgr.GetDefinition(ctx, params.FilePath, params.Line-1, params.Character-1)
	if err != nil {
		return agent.ToolResult{IsError: true, Content: err.Error()}, nil
	}
	locations := extractLocations(results)
	if len(locations) == 0 {
		return agent.ToolResult{Content: "No definition found."}, nil
	}
	return agent.ToolResult{Content: formatLocations(locations, t.workspace)}, nil
}

// ─── 2. lsp_find_references ───────────────────────────────────────────────────

// LSPFindReferencesTool finds all references to the symbol at a position.
type LSPFindReferencesTool struct {
	workspace string
}

func NewLSPFindReferencesTool(workspace string) *LSPFindReferencesTool {
	return &LSPFindReferencesTool{workspace: workspace}
}

func (t *LSPFindReferencesTool) Name() string { return "lsp_find_references" }

func (t *LSPFindReferencesTool) Description() string {
	return "Find all references to the symbol at a specific position in a file using Language Server Protocol. Useful for understanding code impact."
}

func (t *LSPFindReferencesTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath":  map[string]any{"type": "string", "description": "The absolute path to the file to inspect."},
			"line":      map[string]any{"type": "integer", "description": "The 1-based line number (as shown in editors)."},
			"character": map[string]any{"type": "integer", "description": "The 1-based character offset on the line (as shown in editors)."},
		},
		"required": []string{"filePath", "line", "character"},
	}
}

func (t *LSPFindReferencesTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params lspPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if err := validatePositionParams(params); err != nil {
		return nil, err
	}
	return json.Marshal(params)
}

func (t *LSPFindReferencesTool) Execute(ctx context.Context, raw json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var params lspPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	mgr := getLSPManager(t.workspace)
	results, err := mgr.GetReferences(ctx, params.FilePath, params.Line-1, params.Character-1)
	if err != nil {
		return agent.ToolResult{IsError: true, Content: err.Error()}, nil
	}
	locations := extractLocations(results)
	if len(locations) == 0 {
		return agent.ToolResult{Content: "No references found."}, nil
	}
	return agent.ToolResult{Content: formatLocations(locations, t.workspace)}, nil
}

// ─── 3. lsp_hover ─────────────────────────────────────────────────────────────

// LSPHoverTool gets hover information (type info, docs) for a position.
type LSPHoverTool struct {
	workspace string
}

func NewLSPHoverTool(workspace string) *LSPHoverTool {
	return &LSPHoverTool{workspace: workspace}
}

func (t *LSPHoverTool) Name() string { return "lsp_hover" }

func (t *LSPHoverTool) Description() string {
	return "Get hover information (type info, documentation) for a specific position in a file using Language Server Protocol. Useful for understanding types and signatures."
}

func (t *LSPHoverTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath":  map[string]any{"type": "string", "description": "The absolute path to the file to inspect."},
			"line":      map[string]any{"type": "integer", "description": "The 1-based line number (as shown in editors)."},
			"character": map[string]any{"type": "integer", "description": "The 1-based character offset on the line (as shown in editors)."},
		},
		"required": []string{"filePath", "line", "character"},
	}
}

func (t *LSPHoverTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params lspPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if err := validatePositionParams(params); err != nil {
		return nil, err
	}
	return json.Marshal(params)
}

func (t *LSPHoverTool) Execute(ctx context.Context, raw json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var params lspPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	mgr := getLSPManager(t.workspace)
	results, err := mgr.GetHover(ctx, params.FilePath, params.Line-1, params.Character-1)
	if err != nil {
		return agent.ToolResult{IsError: true, Content: err.Error()}, nil
	}
	content := extractHoverContent(results)
	if content == "" {
		return agent.ToolResult{Content: "No hover information found. The position might be on a keyword or the server is still indexing."}, nil
	}
	return agent.ToolResult{Content: content}, nil
}

// ─── 4. lsp_document_symbols ──────────────────────────────────────────────────

// LSPDocumentSymbolsTool lists all symbols in a document.
type LSPDocumentSymbolsTool struct {
	workspace string
}

func NewLSPDocumentSymbolsTool(workspace string) *LSPDocumentSymbolsTool {
	return &LSPDocumentSymbolsTool{workspace: workspace}
}

func (t *LSPDocumentSymbolsTool) Name() string { return "lsp_document_symbols" }

func (t *LSPDocumentSymbolsTool) Description() string {
	return "Get all symbols (functions, classes, variables) in a document using Language Server Protocol. Useful for understanding file structure."
}

func (t *LSPDocumentSymbolsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath": map[string]any{"type": "string", "description": "The absolute path to the file to inspect."},
		},
		"required": []string{"filePath"},
	}
}

func (t *LSPDocumentSymbolsTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params lspFileParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if err := validateFileParams(params); err != nil {
		return nil, err
	}
	return json.Marshal(params)
}

func (t *LSPDocumentSymbolsTool) Execute(ctx context.Context, raw json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var params lspFileParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	mgr := getLSPManager(t.workspace)
	results, err := mgr.GetDocumentSymbols(ctx, params.FilePath)
	if err != nil {
		return agent.ToolResult{IsError: true, Content: err.Error()}, nil
	}
	symbols := extractDocumentSymbols(results)
	if len(symbols) == 0 {
		return agent.ToolResult{Content: "No symbols found."}, nil
	}
	return agent.ToolResult{Content: formatDocumentSymbols(symbols)}, nil
}

// ─── 5. lsp_workspace_symbols ─────────────────────────────────────────────────

// LSPWorkspaceSymbolsTool searches for symbols across the workspace.
type LSPWorkspaceSymbolsTool struct {
	workspace string
}

func NewLSPWorkspaceSymbolsTool(workspace string) *LSPWorkspaceSymbolsTool {
	return &LSPWorkspaceSymbolsTool{workspace: workspace}
}

func (t *LSPWorkspaceSymbolsTool) Name() string { return "lsp_workspace_symbols" }

func (t *LSPWorkspaceSymbolsTool) Description() string {
	return "Search for symbols across the entire workspace using Language Server Protocol. Useful for finding where a function, type, or variable is defined."
}

func (t *LSPWorkspaceSymbolsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "The query string to search for."},
		},
		"required": []string{"query"},
	}
}

func (t *LSPWorkspaceSymbolsTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params lspQueryParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	return json.Marshal(params)
}

func (t *LSPWorkspaceSymbolsTool) Execute(ctx context.Context, raw json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var params lspQueryParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	mgr := getLSPManager(t.workspace)
	results, err := mgr.GetWorkspaceSymbols(ctx, params.Query)
	if err != nil {
		return agent.ToolResult{IsError: true, Content: err.Error()}, nil
	}
	symbols := extractSymbolInformation(results)
	if len(symbols) == 0 {
		return agent.ToolResult{Content: "No symbols found in workspace. The server may still be indexing — retry in a few seconds."}, nil
	}
	return agent.ToolResult{Content: formatSymbolInformation(symbols, t.workspace)}, nil
}

// ─── 6. lsp_go_to_implementation ──────────────────────────────────────────────

// LSPGotoImplementationTool finds implementations of an interface/abstract method.
type LSPGotoImplementationTool struct {
	workspace string
}

func NewLSPGotoImplementationTool(workspace string) *LSPGotoImplementationTool {
	return &LSPGotoImplementationTool{workspace: workspace}
}

func (t *LSPGotoImplementationTool) Name() string { return "lsp_go_to_implementation" }

func (t *LSPGotoImplementationTool) Description() string {
	return "Find implementations of an interface or abstract method at a specific position using Language Server Protocol. Useful for finding concrete implementations."
}

func (t *LSPGotoImplementationTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath":  map[string]any{"type": "string", "description": "The absolute path to the file to inspect."},
			"line":      map[string]any{"type": "integer", "description": "The 1-based line number (as shown in editors)."},
			"character": map[string]any{"type": "integer", "description": "The 1-based character offset on the line (as shown in editors)."},
		},
		"required": []string{"filePath", "line", "character"},
	}
}

func (t *LSPGotoImplementationTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params lspPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if err := validatePositionParams(params); err != nil {
		return nil, err
	}
	return json.Marshal(params)
}

func (t *LSPGotoImplementationTool) Execute(ctx context.Context, raw json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var params lspPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	mgr := getLSPManager(t.workspace)
	results, err := mgr.GetImplementation(ctx, params.FilePath, params.Line-1, params.Character-1)
	if err != nil {
		return agent.ToolResult{IsError: true, Content: err.Error()}, nil
	}
	locations := extractLocations(results)
	if len(locations) == 0 {
		return agent.ToolResult{Content: "No implementations found."}, nil
	}
	return agent.ToolResult{Content: formatLocations(locations, t.workspace)}, nil
}

// ─── Validation helpers ───────────────────────────────────────────────────────

func validatePositionParams(p lspPositionParams) error {
	if p.FilePath == "" || !filepath.IsAbs(p.FilePath) {
		return fmt.Errorf("filePath must be an absolute path")
	}
	if p.Line < 1 || p.Character < 1 {
		return fmt.Errorf("line and character must be 1-based (>= 1)")
	}
	return nil
}

func validateFileParams(p lspFileParams) error {
	if p.FilePath == "" || !filepath.IsAbs(p.FilePath) {
		return fmt.Errorf("filePath must be an absolute path")
	}
	return nil
}

// ─── Result extraction & formatting ───────────────────────────────────────────

// extractLocations flattens JSON-RPC results (each may be a Location, Location[],
// or LocationLink[]) into a unified []Location list.
func extractLocations(results []json.RawMessage) []lsp.Location {
	var out []lsp.Location
	for _, raw := range results {
		// Try single Location first
		var single lsp.Location
		if json.Unmarshal(raw, &single) == nil && single.URI != "" {
			out = append(out, single)
			continue
		}
		// Try Location array — validate that entries actually have URIs
		var arr []lsp.Location
		if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 && arr[0].URI != "" {
			out = append(out, arr...)
			continue
		}
		// Try LocationLink array (targetUri / targetSelectionRange)
		var links []lsp.LocationLink
		if json.Unmarshal(raw, &links) == nil && len(links) > 0 {
			for _, link := range links {
				out = append(out, lsp.Location{
					URI:   link.TargetURI,
					Range: link.TargetSelectionRange,
				})
			}
		}
	}
	return out
}

// formatLocations renders locations as user-friendly text.
// Positions are converted from 0-based (LSP) to 1-based (editor convention).
func formatLocations(locs []lsp.Location, workspace string) string {
	var b strings.Builder
	for _, loc := range locs {
		filePath := lsp.URIToPath(loc.URI)
		displayPath := relPath(workspace, filePath)
		start := loc.Range.Start
		end := loc.Range.End
		if start.Line == end.Line {
			b.WriteString(fmt.Sprintf("- %s:%d:%d-%d\n", displayPath, start.Line+1, start.Character+1, end.Character+1))
		} else {
			b.WriteString(fmt.Sprintf("- %s:%d:%d - %d:%d\n", displayPath, start.Line+1, start.Character+1, end.Line+1, end.Character+1))
		}
	}
	b.WriteString(fmt.Sprintf("\n%d location(s) found.", len(locs)))
	return b.String()
}

// extractHoverContent aggregates hover contents from multiple server results.
func extractHoverContent(results []json.RawMessage) string {
	var parts []string
	for _, raw := range results {
		// Could be null
		if string(raw) == "null" {
			continue
		}
		// Try Hover with MarkupContent
		var hover struct {
			Contents json.RawMessage `json:"contents"`
		}
		if json.Unmarshal(raw, &hover) != nil {
			continue
		}
		if len(hover.Contents) == 0 {
			continue
		}
		// Contents can be: string, MarkupContent{kind,value}, or array of either
		text := parseHoverContents(hover.Contents)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n---\n")
}

// parseHoverContents handles the three forms of Hover.contents in the LSP spec:
// 1. string
// 2. MarkupContent {kind, value}
// 3. MarkedString | MarkedString[] (deprecated, {language, value} or string)
func parseHoverContents(raw json.RawMessage) string {
	// Try plain string
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	// Try MarkupContent
	var mc lsp.MarkupContent
	if json.Unmarshal(raw, &mc) == nil && mc.Value != "" {
		return mc.Value
	}
	// Try array of MarkedString/string
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		var parts []string
		for _, item := range arr {
			var str string
			if json.Unmarshal(item, &str) == nil {
				parts = append(parts, str)
				continue
			}
			var ms struct {
				Value string `json:"value"`
			}
			if json.Unmarshal(item, &ms) == nil && ms.Value != "" {
				parts = append(parts, ms.Value)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

// extractDocumentSymbols flattens document symbol results.
// Servers may return DocumentSymbol[] (hierarchical) or SymbolInformation[] (flat).
func extractDocumentSymbols(results []json.RawMessage) []lsp.DocumentSymbol {
	var out []lsp.DocumentSymbol
	for _, raw := range results {
		// Try DocumentSymbol[]
		var docSymbols []lsp.DocumentSymbol
		if json.Unmarshal(raw, &docSymbols) == nil && len(docSymbols) > 0 {
			out = append(out, docSymbols...)
			continue
		}
		// Try SymbolInformation[] — convert to DocumentSymbol-like
		var symInfos []lsp.SymbolInformation
		if json.Unmarshal(raw, &symInfos) == nil {
			for _, si := range symInfos {
				out = append(out, lsp.DocumentSymbol{
					Name:           si.Name,
					Kind:           si.Kind,
					Range:          si.Location.Range,
					SelectionRange: si.Location.Range,
				})
			}
		}
	}
	return out
}

// formatDocumentSymbols renders document symbols as an indented tree.
func formatDocumentSymbols(symbols []lsp.DocumentSymbol) string {
	var b strings.Builder
	for _, s := range symbols {
		formatDocumentSymbol(&b, &s, "")
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatDocumentSymbol(b *strings.Builder, s *lsp.DocumentSymbol, indent string) {
	pos := s.SelectionRange.Start
	kindName := lsp.SymbolKindName(s.Kind)
	b.WriteString(fmt.Sprintf("%s- [%s] %s (Line %d, Char %d)\n", indent, kindName, s.Name, pos.Line+1, pos.Character+1))
	for i := range s.Children {
		formatDocumentSymbol(b, &s.Children[i], indent+"  ")
	}
}

// extractSymbolInformation flattens workspace symbol results.
func extractSymbolInformation(results []json.RawMessage) []lsp.SymbolInformation {
	var out []lsp.SymbolInformation
	for _, raw := range results {
		var syms []lsp.SymbolInformation
		if json.Unmarshal(raw, &syms) == nil {
			out = append(out, syms...)
		}
	}
	return out
}

// formatSymbolInformation renders workspace symbols.
func formatSymbolInformation(symbols []lsp.SymbolInformation, workspace string) string {
	var b strings.Builder
	for _, s := range symbols {
		filePath := lsp.URIToPath(s.Location.URI)
		displayPath := relPath(workspace, filePath)
		line := s.Location.Range.Start.Line + 1
		kindName := lsp.SymbolKindName(s.Kind)
		b.WriteString(fmt.Sprintf("- [%s] %s in %s:%d\n", kindName, s.Name, displayPath, line))
	}
	b.WriteString(fmt.Sprintf("\n%d symbol(s) found.", len(symbols)))
	return b.String()
}

// relPath returns a relative path if under workspace, otherwise the full path.
func relPath(workspace, path string) string {
	if workspace == "" {
		return path
	}
	rel, err := filepath.Rel(workspace, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}

// ─── Concurrency safety ───────────────────────────────────────────────────────

// IsConcurrencySafe implements agent.ConcurrencySafeChecker.
// LSP tools are safe to run concurrently — the Manager serialises server
// access internally with mutexes.
func (t *LSPGotoDefinitionTool) IsConcurrencySafe(_ json.RawMessage) bool     { return true }
func (t *LSPFindReferencesTool) IsConcurrencySafe(_ json.RawMessage) bool     { return true }
func (t *LSPHoverTool) IsConcurrencySafe(_ json.RawMessage) bool              { return true }
func (t *LSPDocumentSymbolsTool) IsConcurrencySafe(_ json.RawMessage) bool    { return true }
func (t *LSPWorkspaceSymbolsTool) IsConcurrencySafe(_ json.RawMessage) bool   { return true }
func (t *LSPGotoImplementationTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// ─── Prompt info (system prompt snippet/guidelines) ───────────────────────────

func (t *LSPGotoDefinitionTool) PromptSnippet() string { return "Go to symbol definition via LSP" }
func (t *LSPFindReferencesTool) PromptSnippet() string {
	return "Find all references to a symbol via LSP"
}
func (t *LSPHoverTool) PromptSnippet() string            { return "Get hover/type info for a symbol via LSP" }
func (t *LSPDocumentSymbolsTool) PromptSnippet() string  { return "List symbols in a document via LSP" }
func (t *LSPWorkspaceSymbolsTool) PromptSnippet() string { return "Search workspace symbols via LSP" }
func (t *LSPGotoImplementationTool) PromptSnippet() string {
	return "Find implementations of an interface via LSP"
}

func lspGuidelines() []string {
	return []string{
		"LSP tools require a language server (gopls for Go); the first call may be slow as the server starts and indexes",
		"Line and character parameters are 1-based (as shown in editors)",
		"If a query returns no results, the server may still be indexing — retry after a few seconds",
	}
}

func (t *LSPGotoDefinitionTool) PromptGuidelines() []string     { return lspGuidelines() }
func (t *LSPFindReferencesTool) PromptGuidelines() []string     { return lspGuidelines() }
func (t *LSPHoverTool) PromptGuidelines() []string              { return lspGuidelines() }
func (t *LSPDocumentSymbolsTool) PromptGuidelines() []string    { return lspGuidelines() }
func (t *LSPWorkspaceSymbolsTool) PromptGuidelines() []string   { return lspGuidelines() }
func (t *LSPGotoImplementationTool) PromptGuidelines() []string { return lspGuidelines() }

// ─── Factory: NewLSPTools ─────────────────────────────────────────────────────

// NewLSPTools returns all six LSP tools configured for the given workspace.
// This is the convenience entry point for tool registration.
func NewLSPTools(workspace string) []agent.Tool {
	return []agent.Tool{
		NewLSPGotoDefinitionTool(workspace),
		NewLSPFindReferencesTool(workspace),
		NewLSPHoverTool(workspace),
		NewLSPDocumentSymbolsTool(workspace),
		NewLSPWorkspaceSymbolsTool(workspace),
		NewLSPGotoImplementationTool(workspace),
	}
}
