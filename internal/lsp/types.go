// Package lsp implements a Language Server Protocol client manager.
//
// It spawns language server processes (e.g. gopls), communicates with them
// via JSON-RPC 2.0 over stdio, and exposes high-level operations such as
// go-to-definition, find-references, hover, document symbols, workspace
// symbols, and go-to-implementation.
//
// The manager is designed to be extensible: new language servers can be
// registered by implementing the ServerSpec interface and adding it to
// DefaultServers.
package lsp

// ─── LSP Protocol Types ───────────────────────────────────────────────────────
//
// These mirror the Language Server Protocol specification (3.17).
// Only the fields that we actually use are included; all types accept
// extra fields via json.RawMessage-friendly encoding (unknown fields are
// silently ignored by Go's encoding/json).

// Position in a text document expressed as zero-based line and character offset.
// The LSP spec uses 0-based positions; tool-layer conversions to/from 1-based
// happen in internal/tools/lsp.go.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a (start, end) pair in a text document. Start must be <= end.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location identifies a region in a specific document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// LocationLink is a richer form of Location used by some servers for
// go-to-definition / implementation responses.
type LocationLink struct {
	OriginSelectionRange *Range `json:"originSelectionRange,omitempty"`
	TargetURI            string `json:"targetUri"`
	TargetRange          Range  `json:"targetRange"`
	TargetSelectionRange Range  `json:"targetSelectionRange"`
}

// HoverContent is either a raw string or a MarkupContent object.
type HoverContent struct {
	Kind  string `json:"kind,omitempty"`  // "markdown" | "plaintext"
	Value string `json:"value,omitempty"`
}

// Hover represents the result of a textDocument/hover request.
type Hover struct {
	Contents HoverContent `json:"contents"`
	Range    *Range       `json:"range,omitempty"`
}

// MarkupContent is a structured content type used in many LSP responses.
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// SymbolKind enumerates LSP symbol kinds (subset of the 26 defined kinds).
const (
	SymbolKindFile         = 1
	SymbolKindModule       = 2
	SymbolKindNamespace    = 3
	SymbolKindPackage      = 4
	SymbolKindClass        = 5
	SymbolKindMethod       = 6
	SymbolKindProperty     = 7
	SymbolKindField        = 8
	SymbolKindConstructor  = 9
	SymbolKindEnum         = 10
	SymbolKindInterface    = 11
	SymbolKindFunction     = 12
	SymbolKindVariable     = 13
	SymbolKindConstant     = 14
	SymbolKindString       = 15
	SymbolKindNumber       = 16
	SymbolKindBoolean      = 17
	SymbolKindArray        = 18
	SymbolKindObject       = 19
	SymbolKindKey          = 20
	SymbolKindNull         = 21
	SymbolKindEnumMember   = 22
	SymbolKindStruct       = 23
	SymbolKindEvent        = 24
	SymbolKindOperator     = 25
	SymbolKindTypeParameter = 26
)

// DocumentSymbol represents a symbol within a document (hierarchical).
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Tags           []int            `json:"tags,omitempty"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation represents a symbol located in the workspace.
type SymbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Tags          []int    `json:"tags,omitempty"`
	Deprecated    bool     `json:"deprecated,omitempty"`
	Location      Location `json:"location"`
	ContainerName string   `json:"containerName,omitempty"`
}

// SymbolKindName returns a human-readable name for an LSP symbol kind.
func SymbolKindName(kind int) string {
	names := map[int]string{
		SymbolKindFile:          "File",
		SymbolKindModule:        "Module",
		SymbolKindNamespace:     "Namespace",
		SymbolKindPackage:       "Package",
		SymbolKindClass:         "Class",
		SymbolKindMethod:        "Method",
		SymbolKindProperty:      "Property",
		SymbolKindField:         "Field",
		SymbolKindConstructor:   "Constructor",
		SymbolKindEnum:          "Enum",
		SymbolKindInterface:     "Interface",
		SymbolKindFunction:      "Function",
		SymbolKindVariable:      "Variable",
		SymbolKindConstant:      "Constant",
		SymbolKindString:        "String",
		SymbolKindNumber:        "Number",
		SymbolKindBoolean:       "Boolean",
		SymbolKindArray:         "Array",
		SymbolKindObject:        "Object",
		SymbolKindKey:           "Key",
		SymbolKindNull:          "Null",
		SymbolKindEnumMember:    "EnumMember",
		SymbolKindStruct:        "Struct",
		SymbolKindEvent:         "Event",
		SymbolKindOperator:      "Operator",
		SymbolKindTypeParameter: "TypeParameter",
	}
	if name, ok := names[kind]; ok {
		return name
	}
	return "Unknown"
}
