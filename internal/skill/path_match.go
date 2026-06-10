package skill

import (
	"path/filepath"
	"regexp"
	"strings"
)

var pathTokenRegex = regexp.MustCompile("`([^`]+)`|\"([^\"]+)\"|'([^']+)'|([^\\s,;，；]+)")

// MatchByPath returns skills whose frontmatter paths match any path-like token
// in text. It is intentionally conservative: it only matches skills that
// explicitly declare paths.
func MatchByPath(skills []Skill, workspace string, text string) []Skill {
	return MatchByExplicitPaths(skills, workspace, extractPathTokens(text))
}

// MatchByExplicitPaths returns skills whose frontmatter paths match any path in
// paths. Callers that already parsed tool arguments should use this instead of
// asking MatchByPath to tokenize free-form text.
func MatchByExplicitPaths(skills []Skill, workspace string, paths []string) []Skill {
	if len(paths) == 0 {
		return nil
	}
	var matched []Skill
	for _, sk := range skills {
		if len(sk.Paths) == 0 || sk.DisableModelInvocation {
			continue
		}
		if skillMatchesAnyPath(sk, workspace, paths) {
			matched = append(matched, sk)
		}
	}
	return matched
}

func skillMatchesAnyPath(sk Skill, workspace string, paths []string) bool {
	for _, path := range paths {
		for _, pattern := range sk.Paths {
			if pathPatternMatches(pattern, workspace, path) {
				return true
			}
		}
	}
	return false
}

func extractPathTokens(text string) []string {
	matches := pathTokenRegex.FindAllStringSubmatch(text, -1)
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{})
	for _, match := range matches {
		token := ""
		for i := 1; i < len(match); i++ {
			if match[i] != "" {
				token = match[i]
				break
			}
		}
		token = strings.TrimSpace(token)
		token = strings.Trim(token, ".:()[]{}<>")
		if !looksLikePath(token) {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func looksLikePath(token string) bool {
	if token == "" || strings.Contains(token, "://") {
		return false
	}
	if strings.Contains(token, "/") || strings.Contains(token, "\\") {
		return true
	}
	ext := filepath.Ext(token)
	return ext != "" && len(ext) <= 12
}

func pathPatternMatches(pattern, workspace, path string) bool {
	pattern = filepath.Clean(strings.TrimSpace(pattern))
	path = filepath.Clean(strings.TrimSpace(path))
	if filepath.IsAbs(path) && workspace != "" {
		if rel, err := filepath.Rel(workspace, path); err == nil && !strings.HasPrefix(rel, "..") {
			path = filepath.Clean(rel)
		}
	}
	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	if !strings.Contains(pattern, string(filepath.Separator)) {
		if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
			return true
		}
	}
	if filepath.IsAbs(path) && pathSuffixPatternMatches(pattern, path) {
		return true
	}
	if strings.HasSuffix(pattern, string(filepath.Separator)+"**") {
		prefix := strings.TrimSuffix(pattern, string(filepath.Separator)+"**")
		return path == prefix ||
			strings.HasPrefix(path, prefix+string(filepath.Separator)) ||
			strings.HasSuffix(path, string(filepath.Separator)+prefix) ||
			strings.Contains(path, string(filepath.Separator)+prefix+string(filepath.Separator))
	}
	if strings.Contains(pattern, string(filepath.Separator)+"**"+string(filepath.Separator)) {
		parts := strings.SplitN(pattern, string(filepath.Separator)+"**"+string(filepath.Separator), 2)
		if len(parts) == 2 && (path == parts[0] ||
			strings.HasPrefix(path, parts[0]+string(filepath.Separator)) ||
			strings.Contains(path, string(filepath.Separator)+parts[0]+string(filepath.Separator))) {
			suffixPattern := parts[1]
			if ok, _ := filepath.Match(suffixPattern, filepath.Base(path)); ok {
				return true
			}
		}
	}
	return false
}

func pathSuffixPatternMatches(pattern, path string) bool {
	path = strings.TrimPrefix(filepath.Clean(path), string(filepath.Separator))
	parts := strings.Split(path, string(filepath.Separator))
	for i := 0; i < len(parts); i++ {
		suffix := filepath.Join(parts[i:]...)
		if ok, _ := filepath.Match(pattern, suffix); ok {
			return true
		}
	}
	return false
}
