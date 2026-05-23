package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
	platformprompt "github.com/earendil-works/pi-go/internal/prompt"
	"github.com/earendil-works/pi-go/internal/skill"
)

// Options configures the coding-agent system prompt.
type Options struct {
	CustomPrompt       string
	CWD                string
	Tools              []agent.Tool
	ContextFiles       []platformprompt.ContextFile
	Skills             []skill.Skill
	AppendSystemPrompt string
}

// BuildSystemPrompt constructs the coding-agent system prompt on top of shared prompt context.
func BuildSystemPrompt(opts Options) string {
	var b strings.Builder

	base := opts.CustomPrompt
	if base == "" {
		base = defaultPrompt
	}
	b.WriteString(base)
	b.WriteString("\n")

	toolNames := collectToolNames(opts.Tools)
	snippets := collectToolSnippets(opts.Tools)
	guidelines := collectToolGuidelines(opts.Tools, toolNames)

	if len(snippets) > 0 {
		b.WriteString("\n## Tool Summary\n\n")
		for _, name := range toolNames {
			if snippet, ok := snippets[name]; ok {
				b.WriteString(fmt.Sprintf("- **%s**: %s\n", name, snippet))
			}
		}
	}

	if len(opts.Tools) > 0 {
		b.WriteString("\n## Available Tools\n\n")
		for _, tool := range opts.Tools {
			b.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", tool.Name(), tool.Description()))
			if params := tool.Parameters(); len(params) > 0 {
				b.WriteString(formatParameters(tool.Name(), params))
			}
		}
	}

	if len(guidelines) > 0 {
		b.WriteString("\n## Guidelines\n\n")
		for _, g := range guidelines {
			b.WriteString(fmt.Sprintf("- %s\n", g))
		}
	}

	if len(opts.ContextFiles) > 0 {
		b.WriteString("\n# Project Context\n\n")
		b.WriteString("Project-specific instructions and guidelines:\n\n")
		for _, cf := range opts.ContextFiles {
			b.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", cf.Path, cf.Content))
		}
	}

	if skillsPrompt := skill.FormatForSystemPrompt(opts.Skills); skillsPrompt != "" {
		b.WriteString("\n")
		b.WriteString(skillsPrompt)
		b.WriteString("\n")
	}

	if opts.AppendSystemPrompt != "" {
		b.WriteString("\n")
		b.WriteString(opts.AppendSystemPrompt)
		b.WriteString("\n")
	}

	b.WriteString("\n---\n")
	b.WriteString(fmt.Sprintf("Current date: %s\n", time.Now().Format("2006-01-02")))
	if opts.CWD != "" {
		cwd := filepath.ToSlash(opts.CWD)
		b.WriteString(fmt.Sprintf("Current working directory: %s\n", cwd))
	}
	if branch := getCurrentGitBranch(opts.CWD); branch != "" {
		b.WriteString(fmt.Sprintf("Current git branch: %s\n", branch))
	}

	return b.String()
}

const defaultPrompt = `You are Pi Go, a server-side coding agent built in Go. You help users by reading files, executing commands, editing code, and writing new files.

You operate inside an agent loop:
1. Receive a user message
2. Think about what to do
3. Use tools to accomplish the task (you may call multiple tools in parallel when they are independent)
4. Return your response

Be concise, technical, and safe. Always prefer targeted operations over broad changes.`

func collectToolNames(tools []agent.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name())
	}
	return names
}

func collectToolSnippets(tools []agent.Tool) map[string]string {
	snippets := make(map[string]string)
	for _, t := range tools {
		if pi, ok := t.(agent.ToolWithPromptInfo); ok {
			if s := pi.PromptSnippet(); s != "" {
				snippets[t.Name()] = s
			}
		}
	}
	return snippets
}

func collectToolGuidelines(tools []agent.Tool, toolNames []string) []string {
	guidelineSet := make(map[string]bool)
	var guidelines []string

	addGuideline := func(g string) {
		g = strings.TrimSpace(g)
		if g != "" && !guidelineSet[g] {
			guidelineSet[g] = true
			guidelines = append(guidelines, g)
		}
	}

	has := make(map[string]bool)
	for _, name := range toolNames {
		has[name] = true
	}

	if has["bash"] && !has["grep"] && !has["find"] && !has["ls"] {
		addGuideline("Use bash for file operations like ls, rg, find")
	} else if has["bash"] && (has["grep"] || has["find"] || has["ls"]) {
		addGuideline("Prefer grep/find/ls tools over bash for file exploration (faster, more targeted)")
	}

	if has["read"] && has["edit"] {
		addGuideline("Use read to examine files before editing; use edit for precise string replacements")
	}

	if has["write"] {
		addGuideline("Use write only for creating new files or complete rewrites; prefer edit for modifications")
	}

	if has["bash"] {
		addGuideline("Avoid destructive commands (rm -rf, etc.) unless explicitly asked")
	}

	for _, t := range tools {
		if pi, ok := t.(agent.ToolWithPromptInfo); ok {
			for _, g := range pi.PromptGuidelines() {
				addGuideline(g)
			}
		}
	}

	addGuideline("Be concise in your responses")
	addGuideline("Show file paths clearly when working with files")
	addGuideline("Explain your reasoning before taking actions")

	return guidelines
}

func formatParameters(toolName string, params map[string]any) string {
	props, ok := params["properties"].(map[string]any)
	if !ok {
		return ""
	}

	required := make(map[string]bool)
	if req, ok := params["required"].([]string); ok {
		for _, r := range req {
			required[r] = true
		}
	}

	var b strings.Builder
	b.WriteString("Parameters:\n")
	for name, schema := range props {
		schemaMap, ok := schema.(map[string]any)
		if !ok {
			continue
		}
		typeStr, _ := schemaMap["type"].(string)
		desc, _ := schemaMap["description"].(string)
		reqMark := ""
		if required[name] {
			reqMark = " (required)"
		}
		b.WriteString(fmt.Sprintf("  - `%s` (%s)%s", name, typeStr, reqMark))
		if desc != "" {
			b.WriteString(fmt.Sprintf(": %s", desc))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func getCurrentGitBranch(cwd string) string {
	if cwd == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(cwd, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/")
	}
	return ""
}
