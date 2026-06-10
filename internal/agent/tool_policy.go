package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/session"
)

const defaultSkillPolicyMaxViolations = 2
const shellNonSkillCwd = "\x00workspace"

var policyShellInspectionRegex = regexp.MustCompile(`(?i)(^|[\s;&|()])(ls|find|grep|rg|fd|tree|cat|head|tail|sed|awk|wc|du)\b`)

type activeToolPolicy struct {
	name              string
	args              string
	filePath          string
	skillRoot         string
	allowedTools      map[string]struct{}
	bashCommandSpecs  []string
	allowedSkillPaths map[string]struct{}
	blockedSkillPaths map[string]struct{}
	pathPatterns      []string
	branch            string
	executionContext  string
	compactContext    string
	artifacts         map[string]struct{}
	changedFiles      map[string]struct{}
	operations        []string
	changes           []session.SkillResultChange
	maxViolations     int
	violations        int
	lastViolation     string
}

func newActiveToolPolicy(p ToolPolicyActivation) *activeToolPolicy {
	allowedTools := make(map[string]struct{}, len(p.AllowedTools))
	for _, name := range p.AllowedTools {
		name = canonicalToolName(name)
		if name != "" {
			if name == "skill" {
				continue
			}
			allowedTools[name] = struct{}{}
		}
	}
	bashCommandSpecs := parseBashCommandSpecs(append(append([]string(nil), p.AllowedTools...), p.AllowedToolSpecs...))

	allowedSkillPaths := make(map[string]struct{}, len(p.AllowedSkillPaths))
	for _, path := range p.AllowedSkillPaths {
		if path == "" {
			continue
		}
		allowedSkillPaths[filepath.Clean(path)] = struct{}{}
	}
	blockedSkillPaths := make(map[string]struct{}, len(p.BlockedSkillPaths))
	for _, path := range p.BlockedSkillPaths {
		if path == "" {
			continue
		}
		blockedSkillPaths[filepath.Clean(path)] = struct{}{}
	}

	maxViolations := p.MaxViolations
	if maxViolations <= 0 {
		maxViolations = defaultSkillPolicyMaxViolations
	}
	if p.ExecutionContext == "fork" && maxViolations > 1 {
		maxViolations = 1
	}

	return &activeToolPolicy{
		name:              p.Name,
		args:              p.Args,
		filePath:          p.FilePath,
		skillRoot:         filepath.Clean(p.SkillRoot),
		allowedTools:      allowedTools,
		bashCommandSpecs:  bashCommandSpecs,
		allowedSkillPaths: allowedSkillPaths,
		blockedSkillPaths: blockedSkillPaths,
		pathPatterns:      cleanPathPatterns(p.PathPatterns),
		branch:            p.Branch,
		executionContext:  p.ExecutionContext,
		compactContext:    strings.TrimSpace(p.CompactContext),
		artifacts:         make(map[string]struct{}),
		changedFiles:      make(map[string]struct{}),
		maxViolations:     maxViolations,
	}
}

func cleanPathPatterns(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		pattern = strings.Trim(pattern, "`\"'")
		if pattern != "" {
			out = append(out, filepath.Clean(pattern))
		}
	}
	return out
}

func canonicalToolName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if idx := strings.Index(name, "("); idx >= 0 {
		name = strings.TrimSpace(name[:idx])
	}
	name = strings.Trim(name, "`\"'")
	switch name {
	case "read", "write", "edit", "bash", "grep", "find", "ls", "skill":
		return name
	case "glob":
		return "find"
	case "search":
		return "grep"
	default:
		return name
	}
}

func parseBashCommandSpecs(specs []string) []string {
	seen := make(map[string]struct{}, len(specs))
	var out []string
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		lower := strings.ToLower(spec)
		if !strings.HasPrefix(lower, "bash(") || !strings.HasSuffix(spec, ")") {
			continue
		}
		inner := strings.TrimSpace(spec[len("bash(") : len(spec)-1])
		inner = strings.Trim(inner, "`\"'")
		if inner == "" {
			continue
		}
		if _, ok := seen[inner]; ok {
			continue
		}
		seen[inner] = struct{}{}
		out = append(out, inner)
	}
	return out
}

func (a *Agent) ActivateToolPolicy(p ToolPolicyActivation) {
	a.ActivateToolPolicyWithContext(context.Background(), p)
}

func (a *Agent) ActivateToolPolicyWithContext(ctx context.Context, p ToolPolicyActivation) {
	policy := newActiveToolPolicy(p)
	var forkSession *session.Session
	forkSessionPath := ""
	if p.ExecutionContext == "fork" && a.session != nil && a.session.Storage() != nil {
		forkStorage, err := skillForkStorage(ctx, a.session.Storage())
		if err != nil {
			slog.Warn("failed to fork session for skill policy", "skill", p.Name, "error", err)
		} else {
			forkSessionPath = storagePath(forkStorage)
			forkSession = session.New(forkStorage)
			if err := forkSession.InitFromStorage(ctx); err != nil {
				slog.Warn("failed to initialize fork session for skill policy", "skill", p.Name, "error", err)
				_ = forkStorage.Close()
				forkSession = nil
			}
		}
	}

	a.mu.Lock()
	if a.forkSession != nil && a.forkSession.Storage() != nil {
		_ = a.forkSession.Storage().Close()
	}
	policy.retainAvailableTools(a.tools)
	a.activePolicy = policy
	a.forkSession = forkSession
	a.mu.Unlock()

	inv := skillInvocationFromPolicy(p, forkSessionPath)
	if a.session != nil {
		_ = a.session.AppendSkillInvocation(ctx, inv)
	}
	if forkSession != nil {
		_ = forkSession.AppendSkillInvocation(ctx, inv)
	}
}

type emptyForker interface {
	ForkEmpty(ctx context.Context) (session.SessionStorage, error)
}

func skillForkStorage(ctx context.Context, storage session.SessionStorage) (session.SessionStorage, error) {
	if f, ok := storage.(emptyForker); ok {
		return f.ForkEmpty(ctx)
	}
	return storage.Fork(ctx, "")
}

type pathReporter interface {
	Path() string
}

func storagePath(storage session.SessionStorage) string {
	if p, ok := storage.(pathReporter); ok {
		return p.Path()
	}
	return ""
}

func skillInvocationFromPolicy(p ToolPolicyActivation, forkSessionPath string) session.SkillInvocation {
	return session.SkillInvocation{
		Name:              p.Name,
		Args:              p.Args,
		FilePath:          p.FilePath,
		BaseDir:           p.SkillRoot,
		Branch:            p.Branch,
		ExecutionContext:  p.ExecutionContext,
		ForkSessionPath:   forkSessionPath,
		AllowedTools:      append([]string(nil), p.AllowedTools...),
		AllowedToolSpecs:  append([]string(nil), p.AllowedToolSpecs...),
		AllowedSkillPaths: append([]string(nil), p.AllowedSkillPaths...),
		BlockedSkillPaths: append([]string(nil), p.BlockedSkillPaths...),
		PathPatterns:      append([]string(nil), p.PathPatterns...),
		CompactContext:    strings.TrimSpace(p.CompactContext),
	}
}

func (a *Agent) clearToolPolicy() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeForkCancel != nil {
		a.activeForkCancel()
		a.activeForkCancel = nil
	}
	a.activePolicy = nil
	if a.forkSession != nil && a.forkSession.Storage() != nil {
		_ = a.forkSession.Storage().Close()
	}
	a.forkSession = nil
}

func (p *activeToolPolicy) retainAvailableTools(tools map[string]Tool) {
	if p == nil || len(p.allowedTools) == 0 {
		return
	}
	available := make(map[string]struct{}, len(tools))
	for name := range tools {
		available[canonicalToolName(name)] = struct{}{}
	}
	for name := range p.allowedTools {
		if _, ok := available[name]; !ok {
			delete(p.allowedTools, name)
		}
	}
}

func (a *Agent) currentToolPolicy() *activeToolPolicy {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.activePolicy == nil {
		return nil
	}
	return cloneActivePolicy(a.activePolicy)
}

func (a *Agent) hasActiveToolPolicy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activePolicy != nil
}

func (a *Agent) ActiveToolPolicySnapshot() ToolPolicySnapshot {
	p := a.currentToolPolicy()
	if p == nil {
		return ToolPolicySnapshot{Active: false}
	}
	_, writeAllowed := p.allowedTools["write"]
	_, editAllowed := p.allowedTools["edit"]
	_, readAllowed := p.allowedTools["read"]
	_, bashAllowed := p.allowedTools["bash"]
	_, findAllowed := p.allowedTools["find"]
	_, grepAllowed := p.allowedTools["grep"]
	_, lsAllowed := p.allowedTools["ls"]
	writeEditOnly := (writeAllowed || editAllowed) && !readAllowed && !bashAllowed && !findAllowed && !grepAllowed && !lsAllowed
	return ToolPolicySnapshot{
		Active:            true,
		Name:              p.name,
		Args:              p.args,
		FilePath:          p.filePath,
		SkillRoot:         p.skillRoot,
		AllowedTools:      sortedKeys(p.allowedTools),
		BashCommandSpecs:  append([]string(nil), p.bashCommandSpecs...),
		AllowedSkillPaths: sortedKeys(p.allowedSkillPaths),
		BlockedSkillPaths: sortedKeys(p.blockedSkillPaths),
		PathPatterns:      append([]string(nil), p.pathPatterns...),
		Branch:            p.branch,
		ExecutionContext:  p.executionContext,
		Violations:        p.violations,
		MaxViolations:     p.maxViolations,
		LastViolation:     p.lastViolation,
		WriteEditOnly:     writeEditOnly,
		NoToolsAllowed:    len(p.allowedTools) == 0,
	}
}

func cloneActivePolicy(p *activeToolPolicy) *activeToolPolicy {
	if p == nil {
		return nil
	}
	cp := *p
	cp.allowedTools = cloneStringSet(p.allowedTools)
	cp.allowedSkillPaths = cloneStringSet(p.allowedSkillPaths)
	cp.blockedSkillPaths = cloneStringSet(p.blockedSkillPaths)
	cp.artifacts = cloneStringSet(p.artifacts)
	cp.changedFiles = cloneStringSet(p.changedFiles)
	cp.operations = append([]string(nil), p.operations...)
	cp.changes = cloneSkillResultChanges(p.changes)
	cp.bashCommandSpecs = append([]string(nil), p.bashCommandSpecs...)
	cp.pathPatterns = append([]string(nil), p.pathPatterns...)
	return &cp
}

func cloneStringSet(in map[string]struct{}) map[string]struct{} {
	if in == nil {
		return nil
	}
	out := make(map[string]struct{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneSkillResultChanges(in []session.SkillResultChange) []session.SkillResultChange {
	if len(in) == 0 {
		return nil
	}
	return append([]session.SkillResultChange(nil), in...)
}

func (a *Agent) activeForkSkillPolicy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activePolicy != nil && a.activePolicy.executionContext == "fork"
}

func (a *Agent) currentForkSession() *session.Session {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.forkSession
}

func (a *Agent) currentForkMergeInfo() (name, executionContext, forkSessionPath string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.activePolicy != nil {
		name = a.activePolicy.name
		executionContext = a.activePolicy.executionContext
	}
	if a.forkSession != nil && a.forkSession.Storage() != nil {
		forkSessionPath = storagePath(a.forkSession.Storage())
	}
	return name, executionContext, forkSessionPath
}

func (a *Agent) currentSkillArtifacts() (artifacts, changedFiles, operations []string, changes []session.SkillResultChange, summary string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.activePolicy == nil {
		return nil, nil, nil, nil, ""
	}
	return sortedKeys(a.activePolicy.artifacts),
		sortedKeys(a.activePolicy.changedFiles),
		append([]string(nil), a.activePolicy.operations...),
		cloneSkillResultChanges(a.activePolicy.changes),
		skillResultChangeSummary(a.activePolicy.changes)
}

func (a *Agent) recordSkillToolArtifact(toolName string, args json.RawMessage, result ToolResult) {
	if result.IsError {
		return
	}
	switch canonicalToolName(toolName) {
	case "write", "edit":
	default:
		return
	}
	path := firstStringJSONField(args, "path", "file_path")
	details := fileChangeDetailsFromToolResult(result.Details)
	if details != nil && details.Path != "" {
		path = details.Path
	}
	if path == "" {
		return
	}
	a.recordSkillArtifact(path, canonicalToolName(toolName), result.Content, true, details)
}

func (a *Agent) recordSkillArtifact(path, toolName, resultContent string, changed bool, details *FileChangeDetails) {
	path = strings.TrimSpace(strings.Trim(path, `"'`))
	if path == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activePolicy == nil {
		return
	}
	path = normalizeSkillResultPath(path, a.activePolicy)
	if a.activePolicy.artifacts == nil {
		a.activePolicy.artifacts = make(map[string]struct{})
	}
	a.activePolicy.artifacts[path] = struct{}{}
	if changed {
		if a.activePolicy.changedFiles == nil {
			a.activePolicy.changedFiles = make(map[string]struct{})
		}
		a.activePolicy.changedFiles[path] = struct{}{}
	}
	if summary := skillOperationSummary(toolName, path, resultContent); summary != "" && len(a.activePolicy.operations) < 20 {
		a.activePolicy.operations = append(a.activePolicy.operations, summary)
	}
	if change := skillResultChange(toolName, path, resultContent, details); change.Path != "" && len(a.activePolicy.changes) < 20 {
		a.activePolicy.changes = append(a.activePolicy.changes, change)
	}
}

func normalizeSkillResultPath(path string, p *activeToolPolicy) string {
	if p != nil && p.skillRoot != "" {
		path = strings.ReplaceAll(path, "<SKILL_ROOT>", p.skillRoot)
		path = strings.ReplaceAll(path, "${CLAUDE_SKILL_DIR}", p.skillRoot)
		path = strings.ReplaceAll(path, "$CLAUDE_SKILL_DIR", p.skillRoot)
	}
	return filepath.Clean(path)
}

func skillOperationSummary(toolName, path, resultContent string) string {
	toolName = strings.TrimSpace(toolName)
	path = strings.TrimSpace(path)
	resultContent = strings.TrimSpace(resultContent)
	if toolName == "" || path == "" || resultContent == "" {
		return ""
	}
	const limit = 500
	if len(resultContent) > limit {
		resultContent = resultContent[:limit] + "...(truncated)"
	}
	return fmt.Sprintf("%s %s: %s", toolName, path, resultContent)
}

func skillResultChange(toolName, path, resultContent string, details *FileChangeDetails) session.SkillResultChange {
	toolName = strings.TrimSpace(toolName)
	path = strings.TrimSpace(path)
	resultContent = strings.TrimSpace(resultContent)
	if toolName == "" || path == "" || resultContent == "" {
		return session.SkillResultChange{}
	}
	if details != nil {
		if details.Tool != "" {
			toolName = details.Tool
		}
		if details.Path != "" {
			path = details.Path
		}
		summary := strings.TrimSpace(details.Summary)
		if summary == "" {
			summary = firstNonEmptyLine(resultContent)
		}
		return session.SkillResultChange{
			Path:    path,
			Tool:    toolName,
			Summary: truncatePolicyText(summary, 240),
			Diff:    truncatePolicyText(strings.TrimSpace(details.Diff), 20000),
		}
	}
	return session.SkillResultChange{
		Path:    path,
		Tool:    toolName,
		Summary: truncatePolicyText(firstNonEmptyLine(resultContent), 240),
		Diff:    skillResultDiff(toolName, resultContent),
	}
}

func fileChangeDetailsFromToolResult(details any) *FileChangeDetails {
	switch v := details.(type) {
	case nil:
		return nil
	case FileChangeDetails:
		return &v
	case *FileChangeDetails:
		return v
	case map[string]any:
		return fileChangeDetailsFromMap(v)
	default:
		return nil
	}
}

func fileChangeDetailsFromMap(m map[string]any) *FileChangeDetails {
	details := &FileChangeDetails{}
	if v, ok := m["path"].(string); ok {
		details.Path = v
	}
	if v, ok := m["tool"].(string); ok {
		details.Tool = v
	}
	if v, ok := m["operation"].(string); ok {
		details.Operation = v
	}
	if v, ok := m["summary"].(string); ok {
		details.Summary = v
	}
	if v, ok := m["diff"].(string); ok {
		details.Diff = v
	}
	return details
}

func skillResultDiff(toolName, resultContent string) string {
	if canonicalToolName(toolName) != "edit" || !strings.Contains(resultContent, "\n") {
		return ""
	}
	return truncatePolicyText(strings.TrimSpace(resultContent), 2000)
}

func skillResultChangeSummary(changes []session.SkillResultChange) string {
	if len(changes) == 0 {
		return ""
	}
	var b strings.Builder
	limit := len(changes)
	if limit > 10 {
		limit = 10
	}
	for i := 0; i < limit; i++ {
		change := changes[i]
		if change.Path == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("- ")
		if change.Tool != "" {
			b.WriteString(change.Tool)
			b.WriteString(" ")
		}
		b.WriteString(change.Path)
		if change.Summary != "" {
			b.WriteString(": ")
			b.WriteString(change.Summary)
		}
	}
	if len(changes) > limit {
		b.WriteString(fmt.Sprintf("\n- ... %d more changes", len(changes)-limit))
	}
	return b.String()
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return strings.TrimSpace(text)
}

func truncatePolicyText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "...(truncated)"
}

func (a *Agent) appendForkMessage(ctx context.Context, msg ai.Message) {
	forkSession := a.currentForkSession()
	if forkSession == nil {
		return
	}
	_ = forkSession.AppendMessage(ctx, msg)
}

func (a *Agent) toolAllowedByPolicy(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.activePolicy == nil {
		return true
	}
	_, ok := a.activePolicy.allowedTools[canonicalToolName(name)]
	return ok
}

func (a *Agent) recordPolicyViolation(ctxText string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activePolicy == nil {
		return ctxText
	}
	a.activePolicy.violations++
	p := a.activePolicy
	p.lastViolation = ctxText
	if p.violations >= p.maxViolations && strings.Contains(ctxText, "shell file-inspection") {
		delete(p.allowedTools, "bash")
	}
	if p.violations >= p.maxViolations+1 ||
		(p.violations >= p.maxViolations && isSkillExplorationViolation(ctxText)) {
		delete(p.allowedTools, "bash")
		delete(p.allowedTools, "read")
		delete(p.allowedTools, "find")
		delete(p.allowedTools, "grep")
		delete(p.allowedTools, "ls")
	}
	msg := policyViolationMessage(p, ctxText)
	if p.violations >= p.maxViolations {
		a.followUpQueue.Enqueue(ai.NewTextUserMessage(policyStrongSteeringMessage(p)))
	}
	return msg
}

func isSkillExplorationViolation(reason string) bool {
	reason = strings.ToLower(reason)
	return strings.Contains(reason, "reading skill.md again") ||
		strings.Contains(reason, "reads skill.md again") ||
		strings.Contains(reason, "skill directory path") ||
		strings.Contains(reason, "outside the selected branch allowlist") ||
		strings.Contains(reason, "branch-specific files") ||
		strings.Contains(reason, `tool "skill" is not allowed`) ||
		strings.Contains(reason, `tool "find" is not allowed`) ||
		strings.Contains(reason, `tool "grep" is not allowed`) ||
		strings.Contains(reason, `tool "ls" is not allowed`)
}

func policyViolationMessage(p *activeToolPolicy, reason string) string {
	var b strings.Builder
	b.WriteString("Skill execution policy blocked this tool call")
	if p.name != "" {
		b.WriteString(fmt.Sprintf(" for skill %q", p.name))
	}
	b.WriteString(": ")
	b.WriteString(reason)
	b.WriteString(".\n")
	b.WriteString("Continue with the selected skill workflow branch. Do not explore the skill directory. ")
	b.WriteString("Use only the allowed tools and read only exact skill files referenced by that branch.")
	if p.violations >= p.maxViolations {
		b.WriteString("\nThis is a repeated policy violation; the next assistant turn must stop the current exploration path and either use an allowed tool action or ask the user 1-3 concise questions.")
		if len(p.allowedTools) == 0 {
			b.WriteString(" No tools remain allowed for this active skill turn; do not call another tool. Ask the user 1-3 concise questions or return a brief blocked status.")
			return b.String()
		}
		_, bashAllowed := p.allowedTools["bash"]
		_, readAllowed := p.allowedTools["read"]
		if !bashAllowed && readAllowed {
			b.WriteString(" Bash has been removed from the active skill tool policy for this turn; use write/edit/read instead.")
		}
		if !readAllowed {
			b.WriteString(" Read/search/exploration has been terminated for this skill turn; use write or edit now, or ask concise questions.")
		}
	}
	return b.String()
}

func policyStrongSteeringMessage(p *activeToolPolicy) string {
	var b strings.Builder
	b.WriteString("<skill_policy_recovery>\n")
	b.WriteString("Stop the current exploration path now.\n")
	if p.name != "" {
		b.WriteString(fmt.Sprintf("Active skill: %s\n", p.name))
	}
	if p.branch != "" {
		b.WriteString(fmt.Sprintf("Selected branch: %s\n", p.branch))
	} else if len(p.blockedSkillPaths) > 0 {
		b.WriteString("Selected branch: not set. Ask the user to choose the workflow branch before reading branch-specific skill files.\n")
	}
	if p.executionContext == "fork" {
		b.WriteString("Execution context: fork (isolated skill workflow)\n")
	}
	if len(p.allowedTools) > 0 {
		b.WriteString("Allowed tools: ")
		b.WriteString(strings.Join(sortedKeys(p.allowedTools), ", "))
		b.WriteString("\n")
	} else {
		b.WriteString("Allowed tools: none. Do not call another tool in this active skill turn; ask 1-3 concise questions or return a brief blocked status.\n")
	}
	if _, readAllowed := p.allowedTools["read"]; !readAllowed {
		b.WriteString("Read/search/exploration has been terminated for this skill turn. Use write or edit now, or ask 1-3 concise questions.\n")
	}
	if len(p.bashCommandSpecs) > 0 {
		b.WriteString("Allowed bash command patterns:\n")
		for _, spec := range p.bashCommandSpecs {
			b.WriteString("- ")
			b.WriteString(spec)
			b.WriteString("\n")
		}
	}
	b.WriteString("Do not list, find, grep, cat, head, tail, sed, awk, wc, rg, fd, tree, or otherwise inspect the skill directory.\n")
	b.WriteString("Read only exact files named by the selected branch, then write/edit the requested artifact. If you cannot proceed, ask 1-3 concise questions.\n")
	b.WriteString("</skill_policy_recovery>")
	return b.String()
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (p *activeToolPolicy) compactionNote() string {
	if p == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("<active_skill_context>\n")
	if p.name != "" {
		b.WriteString(fmt.Sprintf("Skill: %s\n", p.name))
	}
	if p.branch != "" {
		b.WriteString(fmt.Sprintf("Selected branch: %s\n", p.branch))
	} else if len(p.blockedSkillPaths) > 0 {
		b.WriteString("Selected branch: not set. Ask the user to choose the workflow branch before reading branch-specific skill files.\n")
	}
	if p.executionContext == "fork" {
		b.WriteString("Execution context: fork. Treat this as an isolated skill workflow: keep intermediate exploration out of the main task, use only the selected branch, and return a concise final artifact/result.\n")
	}
	if p.args != "" {
		b.WriteString(fmt.Sprintf("Invocation args: %s\n", p.args))
	}
	if p.skillRoot != "" {
		b.WriteString(fmt.Sprintf("SKILL_ROOT=%s\n", p.skillRoot))
	}
	if len(p.allowedTools) > 0 {
		b.WriteString("Allowed tools: ")
		b.WriteString(strings.Join(sortedKeys(p.allowedTools), ", "))
		b.WriteString("\n")
	}
	if len(p.bashCommandSpecs) > 0 {
		b.WriteString("Allowed bash command patterns:\n")
		for _, spec := range p.bashCommandSpecs {
			b.WriteString("- ")
			b.WriteString(spec)
			b.WriteString("\n")
		}
	}
	if len(p.allowedSkillPaths) > 0 {
		b.WriteString("Allowed exact skill files:\n")
		for _, path := range sortedKeys(p.allowedSkillPaths) {
			b.WriteString("- ")
			b.WriteString(path)
			b.WriteString("\n")
		}
	}
	if len(p.blockedSkillPaths) > 0 {
		b.WriteString("Blocked branch-specific skill files until the matching branch is selected:\n")
		for _, path := range sortedKeys(p.blockedSkillPaths) {
			b.WriteString("- ")
			b.WriteString(path)
			b.WriteString("\n")
		}
	}
	if len(p.pathPatterns) > 0 {
		b.WriteString("Workspace path patterns:\n")
		for _, pattern := range p.pathPatterns {
			b.WriteString("- ")
			b.WriteString(pattern)
			b.WriteString("\n")
		}
	}
	if p.compactContext != "" {
		b.WriteString(p.compactContext)
		if !strings.HasSuffix(p.compactContext, "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString("Continue the selected skill workflow directly. Do not re-read SKILL.md or explore the skill directory.\n")
	if p.executionContext == "fork" {
		b.WriteString("This skill declared context: fork; preserve the isolated workflow boundary after compaction.\n")
	}
	b.WriteString("</active_skill_context>")
	return b.String()
}

func (a *Agent) activeSkillCompactionNote() string {
	p := a.currentToolPolicy()
	if p == nil {
		return ""
	}
	return p.compactionNote()
}

func mergeCompactionInstructions(customInstructions, skillNote string) string {
	customInstructions = strings.TrimSpace(customInstructions)
	skillNote = strings.TrimSpace(skillNote)
	if skillNote == "" {
		return customInstructions
	}
	if customInstructions == "" {
		return "Preserve this active skill context exactly in the summary:\n" + skillNote
	}
	return customInstructions + "\n\nPreserve this active skill context exactly in the summary:\n" + skillNote
}

func appendCompactionNote(summary, skillNote string) string {
	skillNote = strings.TrimSpace(skillNote)
	if skillNote == "" || strings.Contains(summary, "<active_skill_context>") {
		return summary
	}
	if strings.TrimSpace(summary) == "" {
		return skillNote
	}
	return strings.TrimRight(summary, "\n") + "\n\n" + skillNote
}

func (a *Agent) enforceActivePolicy(call ToolCallContext) *ToolResult {
	if blocked := a.enforceActiveToolNamePolicy(call.ToolName); blocked != nil {
		return blocked
	}

	p := a.currentToolPolicy()
	if p == nil {
		return nil
	}

	toolName := canonicalToolName(call.ToolName)
	switch toolName {
	case "read", "grep", "find", "ls", "write", "edit":
		path := ""
		switch toolName {
		case "read", "write", "edit", "grep":
			path = firstStringJSONField(call.Args, "path", "file_path")
		case "find", "ls":
			path = firstStringJSONField(call.Args, "path", "dir")
		}
		if path == "" {
			return nil
		}
		if reason := p.validateSkillPath(path, toolName); reason != "" {
			msg := a.recordPolicyViolation(reason)
			return &ToolResult{Content: msg, IsError: true}
		}
		if reason := p.validateWorkspacePathPattern(path); reason != "" {
			msg := a.recordPolicyViolation(reason)
			return &ToolResult{Content: msg, IsError: true}
		}
	case "bash":
		command := firstStringJSONField(call.Args, "command")
		if command == "" {
			return nil
		}
		if reason := p.validateBashCommandSpec(command); reason != "" {
			msg := a.recordPolicyViolation(reason)
			return &ToolResult{Content: msg, IsError: true}
		}
		if len(p.bashCommandSpecs) == 0 && policyShellInspectionRegex.MatchString(command) {
			msg := a.recordPolicyViolation(fmt.Sprintf("shell file-inspection command %q is not allowed in skill execution", command))
			return &ToolResult{Content: msg, IsError: true}
		}
		if reason := p.validateBashSkillPaths(command); reason != "" {
			msg := a.recordPolicyViolation(reason)
			return &ToolResult{Content: msg, IsError: true}
		}
		if reason := p.validateBashWorkspacePathPatterns(command); reason != "" {
			msg := a.recordPolicyViolation(reason)
			return &ToolResult{Content: msg, IsError: true}
		}
	}
	return nil
}

func (a *Agent) enforceActiveToolNamePolicy(toolName string) *ToolResult {
	p := a.currentToolPolicy()
	if p == nil {
		return nil
	}
	if _, ok := p.allowedTools[canonicalToolName(toolName)]; !ok {
		msg := a.recordPolicyViolation(fmt.Sprintf("tool %q is not allowed while the skill is active", toolName))
		return &ToolResult{Content: msg, IsError: true}
	}
	a.mu.RLock()
	_, registered := a.tools[toolName]
	a.mu.RUnlock()
	if !registered {
		msg := a.recordPolicyViolation(fmt.Sprintf("tool %q is allowed by skill metadata but is not available in this runtime", toolName))
		return &ToolResult{Content: msg, IsError: true}
	}
	return nil
}

func firstStringJSONField(raw json.RawMessage, names ...string) string {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, name := range names {
		if v, ok := obj[name]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func (p *activeToolPolicy) validateSkillPath(path string, toolName string) string {
	clean, underSkillRoot := p.resolveSkillPathForTool(path, toolName)
	if !underSkillRoot {
		return ""
	}
	if toolName == "write" || toolName == "edit" {
		return fmt.Sprintf("%s on skill file path %q is not allowed; skill files are read-only during skill execution", toolName, path)
	}
	if toolName == "ls" || toolName == "find" || toolName == "grep" {
		return fmt.Sprintf("%s on skill directory path %q is not allowed", toolName, path)
	}
	if filepath.Base(clean) == "SKILL.md" {
		return "reading SKILL.md again is not allowed while the skill workflow is active"
	}
	if _, blocked := p.blockedSkillPaths[clean]; blocked {
		if p.branch == "" {
			return fmt.Sprintf("skill file %q belongs to a workflow branch that has not been selected; ask the user to choose the branch before reading branch-specific files", path)
		}
		return fmt.Sprintf("skill file %q is outside the selected branch allowlist", path)
	}
	if len(p.allowedSkillPaths) == 0 && len(p.blockedSkillPaths) == 0 {
		return ""
	}
	if _, ok := p.allowedSkillPaths[clean]; ok {
		return ""
	}
	return fmt.Sprintf("skill file %q is outside the selected branch allowlist", path)
}

func (p *activeToolPolicy) resolveSkillPathForTool(path string, toolName string) (string, bool) {
	switch toolName {
	case "grep":
		if clean, ok := p.resolveExplicitSkillPath(path); ok {
			return clean, true
		}
		return p.resolveKnownSkillRelativePath(path)
	case "write", "edit":
		return p.resolveExplicitSkillPath(path)
	default:
		return p.resolveSkillPath(path)
	}
}

func (p *activeToolPolicy) resolveExplicitSkillPath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, `"'`)
	if path == "" {
		return "", false
	}
	if filepath.IsAbs(path) || strings.Contains(path, "<SKILL_ROOT>") || strings.Contains(path, "CLAUDE_SKILL_DIR") {
		return p.resolveSkillPath(path)
	}
	return "", false
}

func (p *activeToolPolicy) resolveKnownSkillRelativePath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, `"'`)
	if path == "" || filepath.IsAbs(path) ||
		strings.Contains(path, "<SKILL_ROOT>") ||
		strings.Contains(path, "CLAUDE_SKILL_DIR") ||
		!isLikelySkillRelativeReference(path) {
		return "", false
	}
	clean, underSkillRoot := p.resolveSkillPath(path)
	if !underSkillRoot {
		return "", false
	}
	if filepath.Base(clean) == "SKILL.md" {
		return clean, true
	}
	if _, ok := p.allowedSkillPaths[clean]; ok {
		return clean, true
	}
	if _, ok := p.blockedSkillPaths[clean]; ok {
		return clean, true
	}
	return "", false
}

func (p *activeToolPolicy) validateWorkspacePathPattern(path string) string {
	if len(p.pathPatterns) == 0 {
		return ""
	}
	if p.isExplicitOrAllowedSkillPath(path) {
		return ""
	}
	clean := filepath.Clean(strings.Trim(path, `"'`))
	for _, pattern := range p.pathPatterns {
		if pathPatternMatches(pattern, clean) {
			return ""
		}
	}
	return fmt.Sprintf("workspace path %q is outside this skill's declared paths", path)
}

func (p *activeToolPolicy) validateBashCommandSpec(command string) string {
	if len(p.bashCommandSpecs) == 0 {
		return ""
	}
	if bashCommandAllowedBySpecs(command, p.bashCommandSpecs, 0) {
		return ""
	}
	return fmt.Sprintf("bash command %q is outside this skill's allowed Bash(...) command patterns", command)
}

func bashCommandAllowedBySpecs(command string, specs []string, depth int) bool {
	if depth > 3 {
		return false
	}
	segments := splitShellSegments(command)
	if len(segments) == 0 {
		return false
	}
	for _, segment := range segments {
		if !bashSegmentAllowedBySpecs(segment, specs, depth) {
			return false
		}
	}
	return true
}

func bashSegmentAllowedBySpecs(segment string, specs []string, depth int) bool {
	fields := shellFields(strings.TrimSpace(segment))
	start := shellCommandStartIndex(fields)
	if start < 0 {
		return false
	}
	outer := normalizeShellCommand(strings.Join(fields[start:], " "))
	for _, spec := range specs {
		if bashCommandSpecMatches(outer, spec) {
			return true
		}
	}
	if payload, _, ok := shellEvalPayload(fields); ok {
		return bashCommandAllowedBySpecs(payload, specs, depth+1)
	}
	return false
}

func bashCommandSpecMatches(command, spec string) bool {
	spec = normalizeShellCommand(spec)
	if spec == "" {
		return false
	}
	if strings.HasSuffix(spec, ":*") {
		prefix := strings.TrimSpace(strings.TrimSuffix(spec, ":*"))
		return command == prefix || strings.HasPrefix(command, prefix+" ")
	}
	if strings.HasSuffix(spec, "*") && !strings.Contains(spec[:len(spec)-1], "*") {
		prefix := strings.TrimSpace(strings.TrimSuffix(spec, "*"))
		return strings.HasPrefix(command, prefix)
	}
	if strings.Contains(spec, "*") {
		return wildcardSpecMatches(command, spec)
	}
	return command == spec
}

func normalizeShellCommand(command string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(command)), " ")
}

func normalizeShellCommandSpecCandidates(command string) []string {
	return normalizeShellCommandSpecCandidatesDepth(command, 0)
}

func normalizeShellCommandSpecCandidatesDepth(command string, depth int) []string {
	if depth > 3 {
		return nil
	}
	fields := shellFields(strings.TrimSpace(command))
	start := shellCommandStartIndex(fields)
	if start < 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(candidate string) {
		candidate = normalizeShellCommand(candidate)
		if candidate == "" {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	add(strings.Join(fields[start:], " "))
	if payload, _, ok := shellEvalPayload(fields); ok {
		for _, candidate := range normalizeShellCommandSpecCandidatesDepth(payload, depth+1) {
			add(candidate)
		}
	}
	return out
}

func normalizeShellCommandForSpec(command string) string {
	candidates := normalizeShellCommandSpecCandidates(command)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[len(candidates)-1]
}

func wildcardSpecMatches(command, spec string) bool {
	var b strings.Builder
	b.WriteString("^")
	for _, r := range spec {
		if r == '*' {
			b.WriteString(".*")
			continue
		}
		b.WriteString(regexp.QuoteMeta(string(r)))
	}
	b.WriteString("$")
	ok, err := regexp.MatchString(b.String(), command)
	return err == nil && ok
}

func (p *activeToolPolicy) isExplicitOrAllowedSkillPath(path string) bool {
	path = strings.Trim(path, `"'`)
	if strings.Contains(path, "<SKILL_ROOT>") || strings.Contains(path, "CLAUDE_SKILL_DIR") {
		return true
	}
	if filepath.IsAbs(path) {
		_, underSkillRoot := p.resolveSkillPath(path)
		return underSkillRoot
	}
	clean, underSkillRoot := p.resolveSkillPath(path)
	if !underSkillRoot {
		return false
	}
	if _, ok := p.allowedSkillPaths[clean]; ok {
		return true
	}
	return false
}

func pathPatternMatches(pattern, path string) bool {
	pattern = filepath.Clean(pattern)
	path = filepath.Clean(path)
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
		if len(parts) == 2 {
			prefix := parts[0]
			suffixPattern := parts[1]
			ok, _ := filepath.Match(suffixPattern, filepath.Base(path))
			return ok && (path == prefix ||
				strings.HasPrefix(path, prefix+string(filepath.Separator)) ||
				strings.Contains(path, string(filepath.Separator)+prefix+string(filepath.Separator)))
		}
	}
	if strings.HasPrefix(path, string(filepath.Separator)) {
		return strings.HasSuffix(path, string(filepath.Separator)+pattern) || path == pattern
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

func (p *activeToolPolicy) validateBashSkillPaths(command string) string {
	for _, candidate := range p.extractCommandSkillPathCandidates(command) {
		clean := candidate.clean
		if clean == p.skillRoot {
			return fmt.Sprintf("bash command references skill directory path %q, which is not allowed while the skill workflow is active", candidate.display)
		}
		if filepath.Base(clean) == "SKILL.md" {
			return "bash command reads SKILL.md again, which is not allowed while the skill workflow is active"
		}
		if _, blocked := p.blockedSkillPaths[clean]; blocked {
			if p.branch == "" {
				return fmt.Sprintf("bash command references skill file %q from a workflow branch that has not been selected; ask the user to choose the branch before reading branch-specific files", candidate.display)
			}
			return fmt.Sprintf("bash command references skill file %q outside the selected branch allowlist", candidate.display)
		}
		if len(p.allowedSkillPaths) == 0 {
			continue
		}
		if _, ok := p.allowedSkillPaths[clean]; !ok {
			return fmt.Sprintf("bash command references skill file %q outside the selected branch allowlist", candidate.display)
		}
	}
	return ""
}

func (p *activeToolPolicy) validateBashWorkspacePathPatterns(command string) string {
	if len(p.pathPatterns) == 0 {
		return ""
	}
	for _, candidate := range extractCommandWorkspacePathCandidates(command) {
		if p.isExplicitOrAllowedSkillPath(candidate) || isNonFilesystemCommandToken(candidate) {
			continue
		}
		if reason := p.validateWorkspacePathPattern(candidate); reason != "" {
			return "bash command references " + reason
		}
	}
	return ""
}

func isNonFilesystemCommandToken(token string) bool {
	return strings.Contains(token, "://")
}

func (p *activeToolPolicy) resolveSkillPath(path string) (string, bool) {
	if p.skillRoot == "" {
		return "", false
	}
	path = strings.TrimSpace(path)
	path = strings.Trim(path, `"'`)
	path = strings.ReplaceAll(path, "<SKILL_ROOT>", p.skillRoot)
	path = strings.ReplaceAll(path, "${CLAUDE_SKILL_DIR}", p.skillRoot)
	path = strings.ReplaceAll(path, "$CLAUDE_SKILL_DIR", p.skillRoot)

	var clean string
	if filepath.IsAbs(path) {
		clean = filepath.Clean(path)
	} else {
		clean = filepath.Clean(filepath.Join(p.skillRoot, path))
	}
	root := p.skillRoot
	if clean == root {
		return clean, true
	}
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return clean, strings.HasPrefix(clean, prefix)
}

func extractCommandPathCandidates(command string) []string {
	return extractCommandPathCandidatesDepth(command, 0)
}

type commandPathCandidate struct {
	path       string
	fieldIndex int
}

type skillCommandPathCandidate struct {
	display string
	clean   string
}

func extractCommandPathCandidatesDepth(command string, depth int) []string {
	if depth > 3 {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, segment := range splitShellSegments(command) {
		fields := shellFields(segment)
		if len(fields) == 0 {
			continue
		}
		if envCwd, _, ok := segmentEnvChdir(fields); ok {
			if _, ok := seen[envCwd]; !ok {
				seen[envCwd] = struct{}{}
				out = append(out, envCwd)
			}
		}
		shellPayloadIndex := -1
		if payload, payloadIndex, ok := shellEvalPayload(fields); ok {
			shellPayloadIndex = payloadIndex
			for _, candidate := range extractCommandPathCandidatesDepth(payload, depth+1) {
				if _, ok := seen[candidate]; ok {
					continue
				}
				seen[candidate] = struct{}{}
				out = append(out, candidate)
			}
		}
		inlinePayloadIndex := -1
		if payload, payloadIndex, ok := inlineScriptEvalPayload(fields); ok {
			inlinePayloadIndex = payloadIndex
			for _, candidate := range extractInlineScriptPathCandidates(payload) {
				if _, ok := seen[candidate]; ok {
					continue
				}
				seen[candidate] = struct{}{}
				out = append(out, candidate)
			}
		}
		pathCommandIndex := pathArgumentCommandIndex(fields)
		for _, candidate := range extractSegmentPathCandidates(fields, shellPayloadIndex, inlinePayloadIndex, pathCommandIndex) {
			if _, ok := seen[candidate.path]; ok {
				continue
			}
			seen[candidate.path] = struct{}{}
			out = append(out, candidate.path)
		}
	}
	return out
}

func extractCommandWorkspacePathCandidates(command string) []string {
	return extractCommandWorkspacePathCandidatesDepth(command, 0, "")
}

func (p *activeToolPolicy) extractCommandSkillPathCandidates(command string) []skillCommandPathCandidate {
	return p.extractCommandSkillPathCandidatesDepth(command, 0, "")
}

func (p *activeToolPolicy) extractCommandSkillPathCandidatesDepth(command string, depth int, cwd string) []skillCommandPathCandidate {
	if depth > 3 {
		return nil
	}
	seen := make(map[string]struct{})
	var out []skillCommandPathCandidate
	add := func(display, clean string) {
		if clean == "" {
			return
		}
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		out = append(out, skillCommandPathCandidate{display: display, clean: clean})
	}
	for _, segment := range splitShellSegments(command) {
		fields := shellFields(segment)
		if len(fields) == 0 {
			continue
		}
		segmentCwd := cwd
		_, envCwdIndex, hasEnvCwd := segmentEnvChdir(fields)
		if hasEnvCwd {
			segmentCwd = p.resolveShellSkillCwd(cwd, fields[envCwdIndex])
		}

		shellPayloadIndex := -1
		if payload, payloadIndex, ok := shellEvalPayload(fields); ok {
			shellPayloadIndex = payloadIndex
			for _, candidate := range p.extractCommandSkillPathCandidatesDepth(payload, depth+1, segmentCwd) {
				add(candidate.display, candidate.clean)
			}
		}

		inlinePayloadIndex := -1
		if payload, payloadIndex, ok := inlineScriptEvalPayload(fields); ok {
			inlinePayloadIndex = payloadIndex
			for _, candidate := range extractInlineScriptPathCandidates(payload) {
				if clean, ok := p.resolveShellSkillCandidate(segmentCwd, candidate); ok {
					add(candidate, clean)
				}
			}
		}

		pathCommandIndex := pathArgumentCommandIndex(fields)
		_, directoryChangeIndex, hasDirectoryChange := segmentDirectoryChangeField(fields)
		for _, candidate := range extractSegmentPathCandidates(fields, shellPayloadIndex, inlinePayloadIndex, pathCommandIndex) {
			if hasEnvCwd && candidate.fieldIndex == envCwdIndex {
				continue
			}
			if hasDirectoryChange && candidate.fieldIndex == directoryChangeIndex {
				continue
			}
			if clean, ok := p.resolveShellSkillCandidate(segmentCwd, candidate.path); ok {
				add(candidate.path, clean)
			}
		}

		if hasDirectoryChange {
			cwd = p.resolveShellSkillCwd(cwd, fields[directoryChangeIndex])
		}
	}
	return out
}

func extractCommandWorkspacePathCandidatesDepth(command string, depth int, cwd string) []string {
	if depth > 3 {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	for _, segment := range splitShellSegments(command) {
		fields := shellFields(segment)
		if len(fields) == 0 {
			continue
		}
		segmentCwd := cwd
		envCwd, envCwdIndex, hasEnvCwd := segmentEnvChdir(fields)
		if hasEnvCwd {
			add(applyShellWorkingDir(cwd, envCwd))
			segmentCwd = applyShellWorkingDir(cwd, envCwd)
		}

		shellPayloadIndex := -1
		if payload, payloadIndex, ok := shellEvalPayload(fields); ok {
			shellPayloadIndex = payloadIndex
			for _, candidate := range extractCommandWorkspacePathCandidatesDepth(payload, depth+1, segmentCwd) {
				add(candidate)
			}
		}

		inlinePayloadIndex := -1
		if payload, payloadIndex, ok := inlineScriptEvalPayload(fields); ok {
			inlinePayloadIndex = payloadIndex
			for _, candidate := range extractInlineScriptPathCandidates(payload) {
				add(applyShellWorkingDir(segmentCwd, candidate))
			}
		}

		pathCommandIndex := pathArgumentCommandIndex(fields)
		for _, candidate := range extractSegmentPathCandidates(fields, shellPayloadIndex, inlinePayloadIndex, pathCommandIndex) {
			candidateCwd := segmentCwd
			if hasEnvCwd && candidate.fieldIndex == envCwdIndex {
				candidateCwd = cwd
			}
			add(applyShellWorkingDir(candidateCwd, candidate.path))
		}

		if nextCwd, ok := segmentDirectoryChange(fields); ok {
			cwd = applyShellWorkingDir(cwd, nextCwd)
		}
	}
	return out
}

func extractSegmentPathCandidates(fields []string, shellPayloadIndex, inlinePayloadIndex, pathCommandIndex int) []commandPathCandidate {
	var out []commandPathCandidate
	for i, field := range fields {
		if i == shellPayloadIndex || i == inlinePayloadIndex {
			continue
		}
		token := cleanShellField(field)
		redirectionTarget := false
		if target, ok := splitAttachedRedirectionTarget(field); ok {
			token = cleanShellField(target)
			redirectionTarget = true
		}
		if token == "" {
			continue
		}
		if i == pathCommandIndex {
			continue
		}
		previous := ""
		previousRaw := ""
		if i > 0 {
			previousRaw = fields[i-1]
			previous = cleanShellField(fields[i-1])
		}
		if isRedirectionOperator(previousRaw) {
			redirectionTarget = true
		}
		pathCommandArg := pathCommandIndex >= 0 && i > pathCommandIndex
		candidates := commandFieldPathCandidates(token, pathCommandArg, isPathValueFlag(previous), redirectionTarget)
		for _, candidate := range candidates {
			out = append(out, commandPathCandidate{path: candidate, fieldIndex: i})
		}
	}
	return out
}

func applyShellWorkingDir(cwd, path string) string {
	path = strings.TrimSpace(strings.Trim(path, `"'`))
	if path == "" || cwd == "" || filepath.IsAbs(path) ||
		strings.Contains(path, "<SKILL_ROOT>") ||
		strings.Contains(path, "CLAUDE_SKILL_DIR") ||
		isNonFilesystemCommandToken(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(cwd, path))
}

func (p *activeToolPolicy) resolveShellSkillCandidate(cwd, path string) (string, bool) {
	path = cleanShellField(path)
	if path == "" || isNonFilesystemCommandToken(path) {
		return "", false
	}
	if cwd == shellNonSkillCwd && !filepath.IsAbs(path) &&
		!strings.Contains(path, "<SKILL_ROOT>") &&
		!strings.Contains(path, "CLAUDE_SKILL_DIR") {
		return "", false
	}
	if cwd != "" && cwd != shellNonSkillCwd && !filepath.IsAbs(path) &&
		!strings.Contains(path, "<SKILL_ROOT>") &&
		!strings.Contains(path, "CLAUDE_SKILL_DIR") {
		clean := filepath.Clean(filepath.Join(cwd, path))
		if p.pathUnderSkillRoot(clean) {
			return clean, true
		}
		return "", false
	}
	if filepath.IsAbs(path) || strings.Contains(path, "<SKILL_ROOT>") || strings.Contains(path, "CLAUDE_SKILL_DIR") {
		clean, underSkillRoot := p.resolveSkillPath(path)
		return clean, underSkillRoot
	}
	if isLikelySkillRelativeReference(path) {
		clean, underSkillRoot := p.resolveSkillPath(path)
		return clean, underSkillRoot
	}
	return "", false
}

func (p *activeToolPolicy) resolveShellSkillCwd(cwd, path string) string {
	path = cleanShellField(path)
	if path == "" || isNonFilesystemCommandToken(path) {
		return shellNonSkillCwd
	}
	if cwd == shellNonSkillCwd && !filepath.IsAbs(path) &&
		!strings.Contains(path, "<SKILL_ROOT>") &&
		!strings.Contains(path, "CLAUDE_SKILL_DIR") {
		return shellNonSkillCwd
	}
	if cwd != "" && cwd != shellNonSkillCwd && !filepath.IsAbs(path) &&
		!strings.Contains(path, "<SKILL_ROOT>") &&
		!strings.Contains(path, "CLAUDE_SKILL_DIR") {
		clean := filepath.Clean(filepath.Join(cwd, path))
		if p.pathUnderSkillRoot(clean) {
			return clean
		}
		return shellNonSkillCwd
	}
	clean, underSkillRoot := p.resolveSkillPath(path)
	if underSkillRoot && (filepath.IsAbs(path) || strings.Contains(path, "<SKILL_ROOT>") || strings.Contains(path, "CLAUDE_SKILL_DIR")) {
		return clean
	}
	return shellNonSkillCwd
}

func (p *activeToolPolicy) pathUnderSkillRoot(path string) bool {
	if p.skillRoot == "" || path == "" {
		return false
	}
	path = filepath.Clean(path)
	root := filepath.Clean(p.skillRoot)
	if path == root {
		return true
	}
	if !strings.HasSuffix(root, string(filepath.Separator)) {
		root += string(filepath.Separator)
	}
	return strings.HasPrefix(path, root)
}

func isLikelySkillRelativeReference(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || filepath.IsAbs(path) || strings.HasPrefix(path, ".") {
		return false
	}
	first := path
	if idx := strings.IndexRune(path, filepath.Separator); idx >= 0 {
		first = path[:idx]
	}
	switch strings.ToLower(first) {
	case "assets", "references", "scripts", "templates", "examples", "resources", "snippets":
		return true
	default:
		return false
	}
}

func segmentDirectoryChange(fields []string) (string, bool) {
	path, _, ok := segmentDirectoryChangeField(fields)
	return path, ok
}

func segmentDirectoryChangeField(fields []string) (string, int, bool) {
	start := shellCommandStartIndex(fields)
	if start < 0 {
		return "", -1, false
	}
	switch filepath.Base(cleanShellField(fields[start])) {
	case "cd", "pushd":
	default:
		return "", -1, false
	}
	for i := start + 1; i < len(fields); i++ {
		token := cleanShellField(fields[i])
		if token == "" || token == "--" {
			continue
		}
		if strings.HasPrefix(token, "-") {
			continue
		}
		return token, i, true
	}
	return "", -1, false
}

func segmentEnvChdir(fields []string) (string, int, bool) {
	if len(fields) == 0 || filepath.Base(cleanShellField(fields[0])) != "env" {
		return "", -1, false
	}
	for i := 1; i < len(fields); i++ {
		token := cleanShellField(fields[i])
		if token == "" || isEnvAssignmentToken(token) {
			continue
		}
		if token == "--" {
			return "", -1, false
		}
		if strings.HasPrefix(token, "--chdir=") {
			value := strings.TrimSpace(token[len("--chdir="):])
			if value == "" {
				return "", -1, false
			}
			return value, i, true
		}
		if token == "-C" || token == "--chdir" {
			if i+1 >= len(fields) {
				return "", -1, false
			}
			value := cleanShellField(fields[i+1])
			if value == "" {
				return "", -1, false
			}
			return value, i + 1, true
		}
		if strings.HasPrefix(token, "-") {
			if envOptionTakesSeparateValue(token) {
				i++
			}
			continue
		}
		return "", -1, false
	}
	return "", -1, false
}

func splitShellSegments(command string) []string {
	var segments []string
	var b strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			segments = append(segments, s)
		}
		b.Reset()
	}
	for _, r := range command {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			b.WriteRune(r)
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			b.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			b.WriteRune(r)
			continue
		}
		switch r {
		case ';', '&', '|', '(', ')', '\n', '\r':
			flush()
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return segments
}

func shellFields(segment string) []string {
	var fields []string
	var b strings.Builder
	var quote rune
	escaped := false
	inToken := false
	flush := func() {
		if inToken {
			fields = append(fields, b.String())
		}
		b.Reset()
		inToken = false
	}
	for _, r := range segment {
		if escaped {
			b.WriteRune(r)
			inToken = true
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			inToken = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				inToken = true
				continue
			}
			b.WriteRune(r)
			inToken = true
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			inToken = true
			continue
		}
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		b.WriteRune(r)
		inToken = true
	}
	if escaped {
		b.WriteRune('\\')
	}
	flush()
	return fields
}

func shellEvalPayload(fields []string) (string, int, bool) {
	start := shellCommandStartIndex(fields)
	if start < 0 {
		return "", -1, false
	}
	switch filepath.Base(cleanShellField(fields[start])) {
	case "sh", "bash", "zsh":
	default:
		return "", -1, false
	}
	for i := start + 1; i < len(fields); i++ {
		token := cleanShellField(fields[i])
		if token == "--" {
			continue
		}
		if !strings.HasPrefix(token, "-") {
			return "", -1, false
		}
		if shellOptionRunsCommandString(token) {
			if i+1 >= len(fields) {
				return "", -1, false
			}
			return fields[i+1], i + 1, true
		}
	}
	return "", -1, false
}

func shellOptionRunsCommandString(token string) bool {
	if token == "-c" || token == "--command" {
		return true
	}
	return strings.HasPrefix(token, "-") && !strings.HasPrefix(token, "--") && strings.Contains(token[1:], "c")
}

func inlineScriptEvalPayload(fields []string) (string, int, bool) {
	start := shellCommandStartIndex(fields)
	if start < 0 {
		return "", -1, false
	}
	command := filepath.Base(cleanShellField(fields[start]))
	switch {
	case isPythonCommand(command):
		for i := start + 1; i < len(fields); i++ {
			token := cleanShellField(fields[i])
			if token == "--" || !strings.HasPrefix(token, "-") {
				return "", -1, false
			}
			if pythonOptionRunsCodeString(token) {
				if i+1 >= len(fields) {
					return "", -1, false
				}
				return fields[i+1], i + 1, true
			}
		}
	case command == "node" || command == "nodejs":
		for i := start + 1; i < len(fields); i++ {
			token := cleanShellField(fields[i])
			if token == "--" || !strings.HasPrefix(token, "-") {
				return "", -1, false
			}
			if strings.HasPrefix(token, "--eval=") || strings.HasPrefix(token, "--print=") {
				idx := strings.Index(token, "=")
				return token[idx+1:], i, true
			}
			if nodeOptionRunsCodeString(token) {
				if i+1 >= len(fields) {
					return "", -1, false
				}
				return fields[i+1], i + 1, true
			}
		}
	}
	return "", -1, false
}

func isPythonCommand(command string) bool {
	return command == "python" || strings.HasPrefix(command, "python2") || strings.HasPrefix(command, "python3")
}

func pythonOptionRunsCodeString(token string) bool {
	if token == "-c" {
		return true
	}
	return strings.HasPrefix(token, "-") && !strings.HasPrefix(token, "--") && strings.Contains(token[1:], "c")
}

func nodeOptionRunsCodeString(token string) bool {
	return token == "-e" || token == "--eval" || token == "-p" || token == "--print"
}

func extractInlineScriptPathCandidates(code string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, literal := range inlineScriptStringLiterals(code) {
		value := strings.TrimSpace(literal.value)
		if value == "" || !inlineScriptFileContext(code, literal.start, literal.end) ||
			isInlineScriptModeLiteral(value) || isInlineScriptModuleLiteral(code, literal.start, literal.end) {
			continue
		}
		if isLikelyCommandPath(value) || (len(value) >= 2 && isBarePathCandidateValue(value)) {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}

type inlineScriptLiteral struct {
	value string
	start int
	end   int
}

func inlineScriptStringLiterals(code string) []inlineScriptLiteral {
	var out []inlineScriptLiteral
	for i := 0; i < len(code); i++ {
		quote := code[i]
		if quote != '\'' && quote != '"' && quote != '`' {
			continue
		}
		var b strings.Builder
		start := i
		escaped := false
		for j := i + 1; j < len(code); j++ {
			ch := code[j]
			if escaped {
				b.WriteByte(ch)
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				out = append(out, inlineScriptLiteral{value: b.String(), start: start, end: j + 1})
				i = j
				break
			}
			b.WriteByte(ch)
		}
	}
	return out
}

func inlineScriptFileContext(code string, start, end int) bool {
	before := start - 96
	if before < 0 {
		before = 0
	}
	after := end + 48
	if after > len(code) {
		after = len(code)
	}
	window := strings.ToLower(code[before:after])
	for _, marker := range []string{
		"open(", "path(", "pathlib", "writefile", "readfile", "appendfile", "copyfile",
		"mkdir", "mkdtemp", "rename", "unlink", "rmdir", "rm(", "createwritestream",
		"createreadstream", "fs.", ".write_text", ".read_text", ".mkdir", "join(", "resolve(",
	} {
		if strings.Contains(window, marker) {
			return true
		}
	}
	return false
}

func isInlineScriptModeLiteral(value string) bool {
	switch strings.Trim(value, "+bt") {
	case "r", "w", "a", "x":
		return true
	default:
		return false
	}
}

func isInlineScriptModuleLiteral(code string, start, end int) bool {
	before := start - 24
	if before < 0 {
		before = 0
	}
	after := end + 24
	if after > len(code) {
		after = len(code)
	}
	window := strings.ToLower(code[before:after])
	return strings.Contains(window, "require(") ||
		strings.Contains(window, "import(") ||
		strings.Contains(window, "from ")
}

func commandFieldPathCandidates(field string, pathCommand bool, pathFlagValue bool, redirectionTarget bool) []string {
	if field == "" {
		return nil
	}
	if idx := strings.Index(field, "="); idx > 0 && idx < len(field)-1 {
		key := cleanShellField(field[:idx])
		value := cleanShellField(field[idx+1:])
		if isLikelyCommandPath(value) ||
			(isPathAssignmentName(key) && isBarePathCandidateValue(value)) ||
			(pathCommand && !strings.HasPrefix(value, "-")) {
			return []string{value}
		}
	}
	if isLikelyCommandPath(field) {
		return []string{field}
	}
	if pathFlagValue && !strings.HasPrefix(field, "-") && !isShellRedirection(field) {
		return []string{field}
	}
	if redirectionTarget && !strings.HasPrefix(field, "-") && !isShellRedirection(field) && !isFileDescriptorRedirectTarget(field) {
		return []string{field}
	}
	if pathCommand && !strings.HasPrefix(field, "-") && !isShellRedirection(field) {
		return []string{field}
	}
	return nil
}

func cleanShellField(field string) string {
	field = strings.TrimSpace(field)
	field = strings.Trim(field, `"'`)
	field = strings.TrimRight(field, ",")
	return field
}

func isLikelyCommandPath(token string) bool {
	if token == "" || isNonFilesystemCommandToken(token) || isShellRedirection(token) {
		return false
	}
	return strings.Contains(token, "/") ||
		strings.Contains(token, "<SKILL_ROOT>") ||
		strings.Contains(token, "CLAUDE_SKILL_DIR") ||
		strings.HasPrefix(token, ".") ||
		filepath.Ext(token) != ""
}

func isShellRedirection(token string) bool {
	return token == ">" || token == ">>" || token == "<" ||
		strings.Contains(token, ">&") ||
		strings.Contains(token, "<&")
}

func isRedirectionOperator(token string) bool {
	token = strings.TrimSpace(strings.Trim(token, `"'`))
	if token == "" || strings.Contains(token, "<<") || strings.HasPrefix(token, "<SKILL_ROOT>") {
		return false
	}
	if strings.HasPrefix(token, "&>") {
		return token == "&>" || token == "&>>"
	}
	token = strings.TrimLeft(token, "0123456789")
	switch token {
	case ">", ">>", "<", "<>":
		return true
	default:
		return false
	}
}

func splitAttachedRedirectionTarget(token string) (string, bool) {
	token = strings.TrimSpace(strings.Trim(token, `"'`))
	if token == "" || strings.Contains(token, "<<") || strings.HasPrefix(token, "<SKILL_ROOT>") {
		return "", false
	}
	if strings.HasPrefix(token, "&>>") {
		return redirectionTargetSuffix(token[len("&>>"):])
	}
	if strings.HasPrefix(token, "&>") {
		return redirectionTargetSuffix(token[len("&>"):])
	}
	rest := strings.TrimLeft(token, "0123456789")
	for _, op := range []string{">>", "<>", ">", "<"} {
		if strings.HasPrefix(rest, op) {
			return redirectionTargetSuffix(rest[len(op):])
		}
	}
	return "", false
}

func redirectionTargetSuffix(target string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" || isFileDescriptorRedirectTarget(target) {
		return "", false
	}
	return target, true
}

func isFileDescriptorRedirectTarget(target string) bool {
	target = strings.TrimSpace(target)
	return strings.HasPrefix(target, "&")
}

func pathArgumentCommandIndex(fields []string) int {
	start := shellCommandStartIndex(fields)
	if start < 0 {
		return -1
	}
	if isPathArgumentCommand(cleanShellField(fields[start])) {
		return start
	}
	return -1
}

func shellCommandStartIndex(fields []string) int {
	if len(fields) == 0 {
		return -1
	}
	i := 0
	if filepath.Base(cleanShellField(fields[i])) == "env" {
		i++
		for i < len(fields) {
			token := cleanShellField(fields[i])
			if token == "" || isEnvAssignmentToken(token) {
				i++
				continue
			}
			if token == "--" {
				i++
				break
			}
			if !strings.HasPrefix(token, "-") {
				break
			}
			i++
			if envOptionTakesSeparateValue(token) && i < len(fields) {
				i++
			}
		}
	}
	for i < len(fields) && isEnvAssignmentToken(cleanShellField(fields[i])) {
		i++
	}
	if i >= len(fields) {
		return -1
	}
	return i
}

func envOptionTakesSeparateValue(token string) bool {
	switch token {
	case "-C", "--chdir", "-S", "--split-string", "-u", "--unset":
		return true
	default:
		return false
	}
}

func isEnvAssignmentToken(token string) bool {
	idx := strings.Index(token, "=")
	if idx <= 0 {
		return false
	}
	name := token[:idx]
	if name == "" || strings.HasPrefix(name, "-") {
		return false
	}
	for i, r := range name {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func isPathAssignmentName(name string) bool {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" || name == "path" {
		return false
	}
	if strings.Contains(name, "path") || strings.Contains(name, "dir") ||
		strings.Contains(name, "folder") || strings.Contains(name, "file") ||
		strings.Contains(name, "dest") || strings.Contains(name, "target") ||
		strings.Contains(name, "tmp") || strings.Contains(name, "temp") {
		return true
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' }) {
		switch part {
		case "out", "output":
			return true
		}
	}
	return false
}

func isBarePathCandidateValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "-") || isNonFilesystemCommandToken(value) || isShellRedirection(value) {
		return false
	}
	lower := strings.ToLower(value)
	if lower == "true" || lower == "false" {
		return false
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return false
	}
	return !strings.ContainsAny(value, ":=")
}

func isPathValueFlag(token string) bool {
	switch token {
	case "-o", "-C", "--out", "--output", "--dir", "--directory", "--path", "--file", "--dest", "--destination", "--target":
		return true
	default:
		return false
	}
}

func isPathArgumentCommand(command string) bool {
	command = strings.TrimSpace(command)
	command = filepath.Base(command)
	switch command {
	case "cd", "pushd", "mkdir", "cp", "mv", "touch", "tee", "install", "rsync":
		return true
	default:
		return false
	}
}
