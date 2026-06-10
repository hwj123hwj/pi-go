package tools

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/earendil-works/pi-go/internal/agent"
)

var shellInspectionCommandRegex = regexp.MustCompile(`(?i)(^|[\s;&|()])(ls|find|grep|rg|fd|tree|cat|head|tail|sed|awk|wc|du)\b`)

func isSkillExplorationDirectory(path string) bool {
	clean := filepath.Clean(path)
	parts := strings.Split(clean, string(filepath.Separator))
	for i := 0; i < len(parts); i++ {
		if parts[i] != ".claude" {
			continue
		}
		if i == len(parts)-1 {
			return true
		}
		if parts[i+1] == "skills" {
			return true
		}
	}
	return false
}

func skillDirectoryExplorationDenied(path string) (agent.ToolResult, error) {
	msg := fmt.Sprintf(
		"Refusing to explore skill directory %q. Do not list/search the skill directory. Continue by reading only exact files explicitly referenced by the selected skill workflow branch, or ask the user 1-3 concise questions if the branch is unclear.",
		path,
	)
	return agent.ToolResult{Content: msg, IsError: true}, nil
}

func shellInspectionDenied(command string) (agent.ToolResult, error) {
	msg := fmt.Sprintf(
		"Refusing shell file-inspection command %q. Use dedicated read/grep/find/ls tools for file inspection. For skills, do not explore directories; read only exact files explicitly referenced by the selected workflow branch.",
		command,
	)
	return agent.ToolResult{Content: msg, IsError: true}, nil
}

func isShellInspectionCommand(command string) bool {
	return shellInspectionCommandRegex.MatchString(command)
}
