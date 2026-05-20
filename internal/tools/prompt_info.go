package tools

// 此文件为各工具实现 agent.ToolWithPromptInfo 接口，
// 提供系统提示中的 snippet 和 guidelines。

// ─── BashTool ─────────────────────────────────────────────────────────────────

func (t *BashTool) PromptSnippet() string {
	return "Execute shell commands on the server"
}

func (t *BashTool) PromptGuidelines() []string {
	return []string{
		"Run commands with appropriate timeouts for long-running operations",
		"Check command output for errors before proceeding",
	}
}

// ─── ReadTool ─────────────────────────────────────────────────────────────────

func (t *ReadTool) PromptSnippet() string {
	return "Read file contents from disk"
}

func (t *ReadTool) PromptGuidelines() []string {
	return []string{
		"Use read to examine files instead of cat or sed",
		"Read files fully before editing to understand context",
	}
}

// ─── WriteTool ────────────────────────────────────────────────────────────────

func (t *WriteTool) PromptSnippet() string {
	return "Write or overwrite files on disk"
}

func (t *WriteTool) PromptGuidelines() []string {
	return []string{
		"Use write for creating new files or complete rewrites; prefer edit for modifications",
		"Always confirm the file path before writing to avoid overwriting important files",
	}
}

// ─── EditTool ─────────────────────────────────────────────────────────────────

func (t *EditTool) PromptSnippet() string {
	return "Make precise string replacements in files"
}

func (t *EditTool) PromptGuidelines() []string {
	return []string{
		"Use edit for targeted modifications; it is safer and more precise than write for changes",
		"Always read a file before editing to ensure old_string matches exactly",
		"The old_string must be unique in the file; if not, include more surrounding context",
	}
}

// ─── GrepTool ─────────────────────────────────────────────────────────────────

func (t *GrepTool) PromptSnippet() string {
	return "Search file contents using regex patterns"
}

func (t *GrepTool) PromptGuidelines() []string {
	return []string{
		"Prefer grep over bash for searching file contents (more targeted output)",
		"Use include patterns to narrow search scope (e.g. *.go, *.ts)",
	}
}

// ─── FindTool ─────────────────────────────────────────────────────────────────

func (t *FindTool) PromptSnippet() string {
	return "Find files by name pattern"
}

func (t *FindTool) PromptGuidelines() []string {
	return []string{
		"Prefer find over bash for locating files (respects .gitignore-like skip rules)",
		"Use glob patterns like *.go or test_* to filter results",
	}
}

// ─── LsTool ───────────────────────────────────────────────────────────────────

func (t *LsTool) PromptSnippet() string {
	return "List directory contents"
}

func (t *LsTool) PromptGuidelines() []string {
	return []string{
		"Prefer ls over bash for directory listing (structured output)",
		"Use recurse for exploring project structure",
	}
}
