package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hwj123hwj/pi-go/internal/agent"
)

const (
	memorySectionHeader = "## Easy Code Added Memories"
	memoryDirName       = ".pi-go"
	memoryFileName      = "AGENTS.md"
)

// MemoryTool saves a specific piece of information or fact to long-term memory.
// Memory entries are appended to ~/.pi-go/AGENTS.md under a dedicated section.
type MemoryTool struct {
	mu       sync.Mutex
	filePath string // resolved on first use
}

type MemoryParams struct {
	Fact string `json:"fact"`
}

func NewMemoryTool() *MemoryTool {
	return &MemoryTool{}
}

func (t *MemoryTool) Name() string { return "save_memory" }

func (t *MemoryTool) Description() string {
	return `Saves a specific piece of information or fact to your long-term memory.

Use this tool:
- When the user explicitly asks you to remember something (e.g., "Remember that I like pineapple on pizza").
- When the user states a clear, concise fact about themselves, their preferences, or their environment.

Do NOT use this tool:
- To remember conversational context that is only relevant for the current session.
- To save long, complex, or rambling pieces of text.
- If you are unsure whether the information is a fact worth remembering long-term.

Parameters:
- fact (string, required): The specific fact or piece of information to remember.`
}

func (t *MemoryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fact": map[string]any{
				"type":        "string",
				"description": "The specific fact or piece of information to remember. Should be a clear, self-contained statement.",
			},
		},
		"required": []string{"fact"},
	}
}

func (t *MemoryTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params MemoryParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Fact) == "" {
		return nil, fmt.Errorf("fact must be a non-empty string")
	}
	return json.Marshal(params)
}

func (t *MemoryTool) Execute(_ context.Context, raw json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var params MemoryParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	fact := strings.TrimSpace(params.Fact)
	if fact == "" {
		return agent.ToolResult{IsError: true, Content: "fact must be a non-empty string"}, fmt.Errorf("empty fact")
	}

	// Remove leading hyphens that could be mistaken for markdown list items
	fact = strings.TrimLeft(fact, "- ")
	fact = strings.TrimSpace(fact)

	if err := t.addMemoryEntry(fact); err != nil {
		slog.Error("save_memory failed", "error", err)
		return agent.ToolResult{IsError: true, Content: fmt.Sprintf("Failed to save memory: %v", err)}, err
	}

	msg := fmt.Sprintf("Okay, I've remembered that: %q", fact)
	return agent.ToolResult{Content: msg}, nil
}

// getFilePath returns the memory file path, resolving it lazily.
func (t *MemoryTool) getFilePath() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.filePath != "" {
		return t.filePath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	t.filePath = filepath.Join(home, memoryDirName, memoryFileName)
	return t.filePath, nil
}

// addMemoryEntry appends a memory fact to the file.
func (t *MemoryTool) addMemoryEntry(fact string) error {
	filePath, err := t.getFilePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Read existing content
	content, _ := os.ReadFile(filePath) // ignore error if file doesn't exist

	newItem := fmt.Sprintf("- %s", fact)
	text := string(content)

	headerIdx := strings.Index(text, memorySectionHeader)
	if headerIdx == -1 {
		// Header not found — append header + item
		sep := ensureNewlineSeparation(text)
		text += sep + memorySectionHeader + "\n" + newItem + "\n"
	} else {
		// Header found — insert after the last item in the section
		startOfSection := headerIdx + len(memorySectionHeader)
		endOfSection := strings.Index(text[startOfSection:], "\n## ")
		if endOfSection == -1 {
			endOfSection = len(text)
		} else {
			endOfSection += startOfSection
		}

		before := strings.TrimRight(text[:startOfSection], "\r\n")
		section := strings.TrimRight(text[startOfSection:endOfSection], "\r\n")
		after := text[endOfSection:]

		section += "\n" + newItem
		text = before + "\n" + strings.TrimLeft(section, "\r\n") + "\n" + after
	}

	// Ensure trailing newline
	text = strings.TrimRight(text, "\r\n") + "\n"

	return os.WriteFile(filePath, []byte(text), 0o644)
}

// ensureNewlineSeparation returns the appropriate separator to add before new content.
func ensureNewlineSeparation(currentContent string) string {
	if len(currentContent) == 0 {
		return ""
	}
	if strings.HasSuffix(currentContent, "\n\n") || strings.HasSuffix(currentContent, "\r\n\r\n") {
		return ""
	}
	if strings.HasSuffix(currentContent, "\n") || strings.HasSuffix(currentContent, "\r\n") {
		return "\n"
	}
	return "\n\n"
}
