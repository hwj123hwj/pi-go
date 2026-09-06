package prompt

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ContextFile 表示一个项目上下文文件（如 CLAUDE.md、AGENTS.md）。
type ContextFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// contextFileCandidates 按优先级排列的上下文文件名候选列表。
var contextFileCandidates = []string{
	"CLAUDE.md",
	"CLAUDE.MD",
	"AGENTS.md",
	"AGENTS.MD",
}

// LoadContextFiles 从指定目录加载上下文文件。
// 按候选列表顺序查找，找到第一个即返回。
// 如果目录中存在匹配的文件，读取其内容并返回。
func LoadContextFiles(dir string) []ContextFile {
	return loadContextFilesFromDir(dir)
}

// LoadProjectContextFiles 加载项目上下文文件。
// 从 cwd 向上遍历目录树直到根目录，收集所有找到的上下文文件。
// 同时也加载全局的 agentDir 中的上下文文件。
// 返回的文件按从根到叶的顺序排列（最外层在前）。
func LoadProjectContextFiles(cwd string, agentDir string) []ContextFile {
	var files []ContextFile
	seen := make(map[string]bool)

	// 1. 加载全局 agentDir 的上下文文件
	if agentDir != "" {
		globalFiles := loadContextFilesFromDir(agentDir)
		for _, f := range globalFiles {
			if !seen[f.Path] {
				seen[f.Path] = true
				files = append(files, f)
			}
		}
	}

	// 2. 从 cwd 向上遍历到根目录，收集所有上下文文件
	var ancestorFiles []ContextFile
	current := cwd
	for {
		dirFiles := loadContextFilesFromDir(current)
		for _, f := range dirFiles {
			if !seen[f.Path] {
				seen[f.Path] = true
				ancestorFiles = append(ancestorFiles, f)
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break // 到达根目录
		}
		current = parent
	}

	// 反转祖先文件（从根到叶）
	for i := len(ancestorFiles) - 1; i >= 0; i-- {
		files = append(files, ancestorFiles[i])
	}

	return files
}

// loadContextFilesFromDir 从单个目录加载上下文文件。
func loadContextFilesFromDir(dir string) []ContextFile {
	for _, name := range contextFileCandidates {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		return []ContextFile{{Path: path, Content: content}}
	}
	return nil
}

// homeDir 获取用户 home 目录。
func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if runtime.GOOS == "windows" {
		if home := os.Getenv("USERPROFILE"); home != "" {
			return home
		}
	}
	return ""
}
