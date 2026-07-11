package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hwj123hwj/pi-go/internal/slashcmd"
)

// CompletionKind identifies what type of completion is being offered.
type CompletionKind int

const (
	CompletionNone    CompletionKind = iota
	CompletionSlash                  // /command
	CompletionFile                   // @filepath
	CompletionModel                  // Ctrl+P model selector
)

// CompletionItem represents a single autocomplete suggestion.
type CompletionItem struct {
	Label       string // displayed text
	Description string // help text / description
	InsertText  string // text to insert (may differ from label)
}

// CompletionState manages autocomplete state.
type CompletionState struct {
	kind       CompletionKind
	items      []CompletionItem
	selected   int
	visible    bool
	query      string // current filter text (e.g. "mo" from "/mo")
	queryStart int    // position in the input line where the query starts
	theme      *Theme
}

// NewCompletionState creates a new completion state.
func NewCompletionState() CompletionState {
	return CompletionState{
		theme: DefaultTheme(),
	}
}

// IsActive returns true if the completion popup is visible.
func (cm *CompletionState) IsActive() bool {
	return cm.visible && len(cm.items) > 0
}

// SelectedItem returns the currently highlighted item, or nil.
func (cm *CompletionState) SelectedItem() *CompletionItem {
	if !cm.IsActive() || cm.selected >= len(cm.items) {
		return nil
	}
	return &cm.items[cm.selected]
}

// Next moves selection down.
func (cm *CompletionState) Next() {
	if len(cm.items) == 0 {
		return
	}
	cm.selected = (cm.selected + 1) % len(cm.items)
}

// Prev moves selection up.
func (cm *CompletionState) Prev() {
	if len(cm.items) == 0 {
		return
	}
	cm.selected = (cm.selected - 1 + len(cm.items)) % len(cm.items)
}

// Close hides the popup.
func (cm *CompletionState) Close() {
	cm.visible = false
	cm.items = nil
	cm.kind = CompletionNone
	cm.query = ""
}

// TriggerSlash detects if the input line should trigger slash-command completion.
// Returns true if the popup was activated.
func (cm *CompletionState) TriggerSlash(input string, cursorX int, registry *slashcmd.Registry) bool {
	if registry == nil {
		return false
	}

	// Get the text before cursor
	beforeCursor := substringBefore(input, cursorX)

	// Check if we're typing a slash command
	if !strings.HasPrefix(beforeCursor, "/") {
		return false
	}

	// Extract query (everything after "/" on the current "word")
	slashIdx := strings.LastIndex(beforeCursor, "/")
	query := beforeCursor[slashIdx+1:]

	// Only complete the command name (no spaces)
	if strings.Contains(query, " ") {
		cm.Close()
		return false
	}

	cm.kind = CompletionSlash
	cm.query = query
	cm.queryStart = slashIdx

	// Build items from registry
	names := registry.Names()
	var items []CompletionItem
	for _, name := range names {
		if query == "" || strings.HasPrefix(name, query) {
			cmd := registry.Command(name)
			items = append(items, CompletionItem{
				Label:       "/" + name,
				Description: cmd.Description,
				InsertText:  "/" + name,
			})
		}
	}

	if len(items) == 0 {
		cm.Close()
		return false
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})

	cm.items = items
	cm.selected = 0
	cm.visible = true
	return true
}

// TriggerFile detects if the input line should trigger file-path completion.
// Returns true if the popup was activated.
func (cm *CompletionState) TriggerFile(input string, cursorX int, cwd string) bool {
	beforeCursor := substringBefore(input, cursorX)

	// Find @trigger
	atIdx := strings.LastIndex(beforeCursor, "@")
	if atIdx < 0 {
		return false
	}

	// Ensure @ is at word boundary
	if atIdx > 0 {
		ch := beforeCursor[atIdx-1]
		if ch != ' ' && ch != '\t' && ch != '\n' {
			return false
		}
	}

	query := beforeCursor[atIdx+1:]
	cm.kind = CompletionFile
	cm.query = query
	cm.queryStart = atIdx

	// Determine directory to scan
	searchDir := cwd
	prefix := ""
	if query != "" {
		// If query contains a path separator, scan that directory
		if idx := strings.LastIndex(query, "/"); idx >= 0 {
			searchDir = filepath.Join(cwd, query[:idx])
			prefix = query[idx+1:]
		} else {
			prefix = query
		}
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		cm.Close()
		return false
	}

	var items []CompletionItem
	for _, entry := range entries {
		// Skip hidden files unless explicitly typed
		if strings.HasPrefix(entry.Name(), ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		if prefix == "" || strings.HasPrefix(entry.Name(), prefix) {
			icon := "📄"
			if entry.IsDir() {
				icon = "📁"
			}
			// Build insert text relative to the @ trigger
			relPath := entry.Name()
			if query != "" && strings.Contains(query, "/") {
				relPath = query[:strings.LastIndex(query, "/")+1] + entry.Name()
			}
			if entry.IsDir() {
				relPath += "/"
			}
			items = append(items, CompletionItem{
				Label:      icon + " " + entry.Name(),
				InsertText: "@" + relPath,
			})
		}
	}

	if len(items) == 0 {
		cm.Close()
		return false
	}

	// Limit to 20 items
	if len(items) > 20 {
		items = items[:20]
	}

	cm.items = items
	cm.selected = 0
	cm.visible = true
	return true
}

// TriggerModel populates the popup with available models.
func (cm *CompletionState) TriggerModel(models []ModelOption) bool {
	cm.kind = CompletionModel
	cm.query = ""

	var items []CompletionItem
	for _, m := range models {
		items = append(items, CompletionItem{
			Label:       m.Provider + "/" + m.ModelID,
			Description: m.Description,
			InsertText:  m.Provider + "/" + m.ModelID,
		})
	}

	if len(items) == 0 {
		cm.Close()
		return false
	}

	cm.items = items
	cm.selected = 0
	cm.visible = true
	return true
}

// ModelOption is a simplified model descriptor for the selector popup.
type ModelOption struct {
	Provider    string
	ModelID     string
	Description string
}

// Kind returns the current completion kind.
func (cm *CompletionState) Kind() CompletionKind {
	return cm.kind
}

// Items returns the current suggestion list.
func (cm *CompletionState) Items() []CompletionItem {
	return cm.items
}

// SelectedIndex returns the current selection index.
func (cm *CompletionState) SelectedIndex() int {
	return cm.selected
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// substringBefore returns the input string up to the cursorX-th rune.
func substringBefore(s string, cursorX int) string {
	runes := []rune(s)
	if cursorX >= len(runes) {
		return s
	}
	return string(runes[:cursorX])
}
