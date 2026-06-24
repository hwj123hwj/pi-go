package kbtools

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ── Data Types ────────────────────────────────────────────────────────────

// Entry represents a single knowledge document in the repository.
type Entry struct {
	Path     string   // absolute path
	RelPath  string   // path relative to repo root
	Title    string   // first heading or frontmatter title
	Category string   // from frontmatter, or top-level directory name
	Tags     []string // from frontmatter or #tags in body
	Summary  string   // first paragraph after title, or frontmatter summary
	Modified time.Time
}

// Index is the in-memory representation of the entire repository.
type Index struct {
	Entries []Entry
	Root    string
}

// indexCache caches the index for a short duration to avoid repeated scans.
var (
	cachedIndex    *Index
	cachedAt       time.Time
	cacheMu        sync.Mutex
	cacheDuration  = 30 * time.Second
)

// GetIndex builds (or returns cached) an index of all .md files in the repo.
func GetIndex(repoPath string) (*Index, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if cachedIndex != nil && time.Since(cachedAt) < cacheDuration {
		if cachedIndex.Root == repoPath {
			return cachedIndex, nil
		}
	}

	idx := &Index{Root: repoPath}
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Only index .md files
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		// Skip README at root (it's the repo description, not a knowledge entry)
		relPath, _ := filepath.Rel(repoPath, path)
		if relPath == "README.md" {
			return nil
		}

		entry := parseEntry(path, repoPath)
		entry.RelPath = relPath
		idx.Entries = append(idx.Entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cachedIndex = idx
	cachedAt = time.Now()
	return idx, nil
}

// ClearCache invalidates the index cache (e.g. after a kb_save).
func ClearCache() {
	cacheMu.Lock()
	cachedIndex = nil
	cachedAt = time.Time{}
	cacheMu.Unlock()
}

// ── Parsing ───────────────────────────────────────────────────────────────

// parseEntry reads a markdown file and extracts metadata.
// It supports two formats:
//   - YAML frontmatter (---\ntitle: ...\ntags: [a, b]\n---)
//   - Legacy markdown (# Title\n\nbody...)
func parseEntry(absPath, repoPath string) Entry {
	info, _ := os.Stat(absPath)
	entry := Entry{
		Path:     absPath,
		Modified: modTime(info),
	}

	// Category from top-level directory
	dir := filepath.Dir(absPath)
	if relDir, err := filepath.Rel(repoPath, dir); err == nil && relDir != "." {
		parts := strings.SplitN(relDir, string(filepath.Separator), 2)
		entry.Category = parts[0]
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return entry
	}
	text := string(content)

	// Try YAML frontmatter first
	if fm, body, ok := parseFrontmatter(text); ok {
		if fm["title"] != "" {
			entry.Title = unquote(fm["title"])
		}
		if fm["summary"] != "" {
			entry.Summary = unquote(fm["summary"])
		}
		if fm["category"] != "" {
			entry.Category = unquote(fm["category"])
		}
		if tags, ok := fm["tags"]; ok {
			entry.Tags = parseTagsValue(tags)
		}
		// If title not in frontmatter, extract from body
		if entry.Title == "" {
			entry.Title = extractTitle(body)
		}
		// If summary not in frontmatter, extract from body
		if entry.Summary == "" {
			entry.Summary = extractSummary(body)
		}
		// Extract #tags from body if no frontmatter tags
		if len(entry.Tags) == 0 {
			entry.Tags = extractInlineTags(body)
		}
		return entry
	}

	// No frontmatter — parse legacy format
	entry.Title = extractTitle(text)
	entry.Summary = extractSummary(text)
	entry.Tags = extractInlineTags(text)
	return entry
}

// parseFrontmatter extracts YAML frontmatter (key: value) from the top of a markdown file.
// Returns (metadata map, body without frontmatter, true) if frontmatter exists.
func parseFrontmatter(text string) (map[string]string, string, bool) {
	// Must start with ---
	trimmed := strings.TrimLeft(text, "\n\r\t ")
	if !strings.HasPrefix(trimmed, "---") {
		return nil, text, false
	}
	lines := strings.SplitN(trimmed, "\n", -1)
	if len(lines) < 2 {
		return nil, text, false
	}

	// Find closing ---
	endLine := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endLine = i
			break
		}
	}
	if endLine == -1 {
		return nil, text, false
	}

	meta := make(map[string]string)
	for i := 1; i < endLine; i++ {
		line := lines[i]
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		meta[key] = val
	}

	body := strings.Join(lines[endLine+1:], "\n")
	return meta, body, true
}

// parseTagsValue parses a tags field value like "Go, concurrency" or "[Go, concurrency]".
func parseTagsValue(raw string) []string {
	raw = strings.Trim(raw, "[]")
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`)
		if p != "" {
			tags = append(tags, p)
		}
	}
	return tags
}

// extractTitle finds the first markdown heading.
func extractTitle(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	// Fallback: first non-empty line
	scanner2 := bufio.NewScanner(strings.NewReader(text))
	for scanner2.Scan() {
		line := strings.TrimSpace(scanner2.Text())
		if line != "" && !strings.HasPrefix(line, "---") {
			return line
		}
	}
	return "(无标题)"
}

// extractSummary returns the first non-heading, non-empty paragraph.
func extractSummary(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	var lines []string
	started := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip headings, frontmatter markers, empty lines
		if strings.HasPrefix(line, "#") || line == "---" || line == "" {
			if started {
				break // end of paragraph
			}
			continue
		}
		started = true
		lines = append(lines, line)
		if len(lines) >= 3 {
			break
		}
	}
	summary := strings.Join(lines, " ")
	if len(summary) > 150 {
		summary = summary[:150] + "..."
	}
	return summary
}

// extractInlineTags finds #tag patterns in text (not markdown headings).
func extractInlineTags(text string) []string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	var tags []string
	seen := make(map[string]bool)
	for scanner.Scan() {
		line := scanner.Text()
		// Look for **Tags**: go, concurrency  or  **标签**: go
		lower := strings.ToLower(line)
		if strings.Contains(lower, "**tags**") || strings.Contains(lower, "**标签**") {
			idx := strings.Index(line, ":")
			if idx != -1 {
				for _, tag := range strings.Split(line[idx+1:], ",") {
					tag = strings.TrimSpace(tag)
					tag = strings.Trim(tag, "`*")
					if tag != "" && !seen[tag] {
						tags = append(tags, tag)
						seen[tag] = true
					}
				}
			}
		}
		// Inline #tags at end of line: word #tag1 #tag2
		words := strings.Fields(line)
		for _, w := range words {
			if len(w) > 1 && strings.HasPrefix(w, "#") && !strings.HasPrefix(w, "##") {
				tag := strings.TrimPrefix(w, "#")
				tag = strings.TrimRight(tag, ".,;!?")
				// Must look like a tag (letters/numbers), not a heading
				if isTagLike(tag) && !seen[tag] {
					tags = append(tags, tag)
					seen[tag] = true
				}
			}
		}
	}
	return tags
}

// isTagLike checks if a string looks like a tag (alphanumeric, hyphen, underscore).
func isTagLike(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// modTime returns the modification time, or zero if info is nil.
func modTime(info os.FileInfo) time.Time {
	if info == nil {
		return time.Time{}
	}
	return info.ModTime()
}

// unquote removes surrounding quotes from a YAML scalar value.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
