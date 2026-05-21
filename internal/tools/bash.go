package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
)

type BashTool struct {
	workspace    string // 工作目录限制，空字符串表示不限制
	maxOutputLen int    // 最大输出长度，0 表示使用 DefaultMaxOutputLen
}

type BashParams struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// BashToolOption configures a BashTool during construction.
type BashToolOption func(*BashTool)

// WithBashWorkspace sets the working directory for command execution.
func WithBashWorkspace(ws string) BashToolOption {
	return func(t *BashTool) { t.workspace = ws }
}

// WithBashMaxOutputLen sets the max output truncation length.
func WithBashMaxOutputLen(n int) BashToolOption {
	return func(t *BashTool) { t.maxOutputLen = n }
}

// NewBashTool creates BashTool with optional configuration.
func NewBashTool(opts ...BashToolOption) *BashTool {
	t := &BashTool{}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Description() string { return "Execute a shell command on the server." }
func (t *BashTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{"type": "string", "description": "The shell command to execute."},
			"timeout": map[string]any{"type": "integer", "description": "Timeout in seconds (default 30)."},
		},
		"required": []string{"command"},
	}
}
func (t *BashTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params BashParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.Command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if params.Timeout <= 0 {
		params.Timeout = 30
	}
	return json.Marshal(params)
}
func (t *BashTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	var params BashParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "sh", "-c", params.Command)
	if t.workspace != "" {
		cmd.Dir = t.workspace
	}
	out, err := cmd.CombinedOutput()

	output := string(out)

	// Detect binary output
	if isBinaryOutput(output) {
		return agent.ToolResult{
			Content: fmt.Sprintf("Command produced binary output (%d bytes). Use file redirection to save output.", len(out)),
			IsError: false,
		}, nil
	}

	// Clean ANSI escape sequences
	output = stripANSI(output)

	// Truncate output
	output = TruncateOutput(output, t.maxOutputLen)

	if err != nil {
		return agent.ToolResult{Content: output, IsError: true}, err
	}
	return agent.ToolResult{Content: output}, nil
}

// ansiRegex matches ANSI escape sequences.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// isBinaryOutput checks if the output appears to be binary data.
func isBinaryOutput(s string) bool {
	// Check for null bytes in first 512 bytes
	checkLen := len(s)
	if checkLen > 512 {
		checkLen = 512
	}
	for i := 0; i < checkLen; i++ {
		if s[i] == 0 {
			return true
		}
	}
	return false
}
