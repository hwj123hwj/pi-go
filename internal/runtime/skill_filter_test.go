package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/ai/providers"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/session"
	"github.com/earendil-works/pi-go/internal/skill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillSourceMerge_RetainsUserSkills(t *testing.T) {
	skills := []skill.Skill{
		{Name: "local-skill", Source: skill.SourceProject, FilePath: "/Users/dev/project/.claude/skills/local-skill/SKILL.md"},
		{Name: "global-skill", Source: skill.SourceUser, FilePath: "/Users/dev/.claude/skills/global-skill/SKILL.md"},
	}

	result := skill.MergeByPriority(skills)
	assert.Len(t, result, 2)
	assert.Equal(t, "local-skill", result[0].Name)
	assert.Equal(t, "global-skill", result[1].Name)
}

func TestSkillSourceMerge_ProjectOverridesUserByName(t *testing.T) {
	skills := []skill.Skill{
		{Name: "my-skill", Source: skill.SourceUser, FilePath: "/Users/dev/.claude/skills/my-skill/SKILL.md"},
		{Name: "my-skill", Source: skill.SourceProject, FilePath: "/Users/dev/project/.claude/skills/my-skill/SKILL.md"},
	}

	result := skill.MergeByPriority(skills)
	assert.Len(t, result, 1)
	assert.Equal(t, "my-skill", result[0].Name)
	assert.Equal(t, skill.SourceProject, result[0].Source)
}

func TestSkillSourceMerge_PreferShorterPathWithinSameSource(t *testing.T) {
	skills := []skill.Skill{
		{Name: "dup", Source: skill.SourceProject, FilePath: "/Users/dev/project/.claude/skills/dup/nested/SKILL.md"},
		{Name: "dup", Source: skill.SourceProject, FilePath: "/Users/dev/project/.claude/skills/dup/SKILL.md"},
	}

	result := skill.MergeByPriority(skills)
	assert.Len(t, result, 1)
	assert.Equal(t, "/Users/dev/project/.claude/skills/dup/SKILL.md", result[0].FilePath)
}

func TestClassifySkillSource(t *testing.T) {
	cwd := filepath.Join(t.TempDir(), "project")
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, skill.SourceProject, classifySkillSource(filepath.Join(cwd, ".claude", "skills"), cwd))
	assert.Equal(t, skill.SourceUser, classifySkillSource(filepath.Join(home, ".claude", "skills"), cwd))
	assert.Equal(t, skill.SourceUser, classifySkillSource(filepath.Join(home, ".agents", "skills"), cwd))
	assert.Equal(t, skill.SourceSystem, classifySkillSource(filepath.Join(home, ".codex", "skills", ".system"), cwd))
	assert.Equal(t, skill.SourcePlugin, classifySkillSource(filepath.Join(home, ".codex", "plugins", "cache", "bundle", "skills"), cwd))
	assert.Equal(t, skill.SourceProject, classifySkillSource(filepath.Join(cwd, "packages", "app", ".claude", "skills"), cwd))
	assert.Equal(t, skill.SourceProject, classifySkillSource(filepath.Join(cwd, "packages", "app", ".agents", "skills"), cwd))
}

func TestDefaultSkillDirsDiscoversProjectUserManagedSystemAndPluginSources(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	cwd := filepath.Join(root, "repo")
	t.Setenv("HOME", home)

	expected := []string{
		filepath.Join(cwd, ".claude", "skills"),
		filepath.Join(cwd, ".agents", "skills"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".codex", "skills", ".system"),
		filepath.Join(home, ".codex", "plugins", "cache", "openai-bundled", "browser", "1.0.0", "skills"),
		filepath.Join(home, ".codex", "plugins", "cache", "openai-primary-runtime", "presentations", "2.0.0", "skills"),
	}
	for _, dir := range expected {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".codex", "plugins", "cache", "openai-bundled", "browser", "1.0.0", "node_modules", "pkg", "skills"), 0o755))

	dirs := defaultSkillDirs(cwd)
	assert.Equal(t, expected, dirs)

	sources := skillSourcesForDirs(cwd, nil)
	require.Len(t, sources, len(expected))
	assert.Equal(t, skill.SourceProject, sources[0].Source)
	assert.Equal(t, skill.SourceProject, sources[1].Source)
	assert.Equal(t, skill.SourceUser, sources[2].Source)
	assert.Equal(t, skill.SourceUser, sources[3].Source)
	assert.Equal(t, skill.SourceManaged, sources[4].Source)
	assert.Equal(t, skill.SourceSystem, sources[5].Source)
	assert.Equal(t, skill.SourcePlugin, sources[6].Source)
	assert.Equal(t, skill.SourcePlugin, sources[7].Source)
}

func TestSkillSourceRegistryForDirsUsesLocalProviderAndPriorityMerge(t *testing.T) {
	cwd := t.TempDir()
	projectDir := filepath.Join(cwd, ".claude", "skills")
	userDir := filepath.Join(t.TempDir(), ".claude", "skills")
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "dup"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(userDir, "dup"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "dup", "SKILL.md"), []byte("---\nname: dup\ndescription: Project\n---\nProject"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "dup", "SKILL.md"), []byte("---\nname: dup\ndescription: User\n---\nUser"), 0o644))

	registry := skillSourceRegistryForDirs(cwd, []string{userDir, projectDir})
	require.NotNil(t, registry)
	result := registry.LoadMerged(context.Background())
	require.Len(t, result.Skills, 1)
	assert.Equal(t, skill.SourceProject, result.Skills[0].Source)
	assert.Equal(t, "Project", result.Skills[0].Description)
}

func TestSkillSourceRegistryIncludesConfiguredRemoteSources(t *testing.T) {
	cwd := t.TempDir()
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
	cfg := config.Config{
		DataDir: t.TempDir(),
		RemoteSkillSources: []config.RemoteSkillSource{{
			Name:     "bad-remote",
			Endpoint: "://bad-url",
			Token:    "secret",
		}},
	}

	registry := skillSourceRegistry(cwd, nil, cfg)
	require.NotNil(t, registry)
	result := registry.LoadMerged(context.Background())

	assert.Empty(t, result.Skills)
	assert.Contains(t, diagnosticCodes(result.Diagnostics), skill.DiagReadFailed)
}

func TestSkillSourceRegistryIncludesConfiguredMCPStdioSources(t *testing.T) {
	cwd := t.TempDir()
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
	cfg := config.Config{
		DataDir: t.TempDir(),
		MCPSkillSources: []config.MCPSkillSource{{
			Name:      "missing-mcp",
			Transport: "stdio",
			Command:   filepath.Join(t.TempDir(), "missing-server"),
		}},
	}

	registry := skillSourceRegistry(cwd, nil, cfg)
	require.NotNil(t, registry)
	result := registry.LoadMerged(context.Background())

	assert.Empty(t, result.Skills)
	assert.Contains(t, diagnosticCodes(result.Diagnostics), skill.DiagListFailed)
}

func TestSkillSourceRegistryIncludesConfiguredMCPHTTPSources(t *testing.T) {
	cwd := t.TempDir()
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
	cfg := config.Config{
		DataDir: t.TempDir(),
		MCPSkillSources: []config.MCPSkillSource{{
			Name:      "bad-http-mcp",
			Transport: "streamable-http",
			Endpoint:  "://bad-url",
		}},
	}

	registry := skillSourceRegistry(cwd, nil, cfg)
	require.NotNil(t, registry)
	result := registry.LoadMerged(context.Background())

	assert.Empty(t, result.Skills)
	assert.Contains(t, diagnosticCodes(result.Diagnostics), skill.DiagListFailed)
}

func TestSkillSourceCacheDirDefaultsToDataDir(t *testing.T) {
	cfg := config.Config{DataDir: "/tmp/pi-data"}
	assert.Equal(t, filepath.Join("/tmp/pi-data", "skill-source-cache"), skillSourceCacheDir(cfg))

	cfg.SkillSourceCacheDir = "/tmp/custom-cache"
	assert.Equal(t, "/tmp/custom-cache", skillSourceCacheDir(cfg))
}

func TestSkillReadRoots_IncludesSourceRoots(t *testing.T) {
	skills := []skill.Skill{
		{Name: "a", SourceRoot: "/Users/dev/.claude/skills"},
		{Name: "b", SourceRoot: "/Users/dev/.claude/skills"},
		{Name: "c", BaseDir: "/repo/.claude/skills/c"},
	}

	assert.Equal(t, []string{"/Users/dev/.claude/skills", "/repo/.claude/skills/c"}, skillReadRoots(skills))
}

func diagnosticCodes(diags []skill.Diagnostic) []skill.DiagnosticCode {
	out := make([]skill.DiagnosticCode, 0, len(diags))
	for _, diag := range diags {
		out = append(out, diag.Code)
	}
	return out
}

type runtimeScriptedResponse struct {
	text      string
	toolCalls []ai.ToolCall
	stop      ai.StopReason
}

type runtimeScriptedProvider struct {
	mu        sync.Mutex
	responses []runtimeScriptedResponse
	requests  []ai.StreamRequest
	index     int
}

func (p *runtimeScriptedProvider) Name() string { return "runtime_scripted" }

func (p *runtimeScriptedProvider) StreamSimple(ctx context.Context, req ai.SimpleStreamRequest) (*ai.EventStream, error) {
	return p.Stream(ctx, ai.StreamRequest{
		Model:     req.Model,
		Messages:  req.Messages,
		System:    req.System,
		Tools:     req.Tools,
		MaxTokens: req.MaxTokens,
	})
}

func (p *runtimeScriptedProvider) Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	idx := p.index
	p.index++
	p.mu.Unlock()

	resp := runtimeScriptedResponse{text: "done", stop: ai.StopReasonStop}
	if idx < len(p.responses) {
		resp = p.responses[idx]
	}
	stream := ai.NewEventStream(8)
	go func() {
		defer stream.Close()
		partial := ai.StreamAssistantMessage{Text: resp.text, ToolCalls: resp.toolCalls, StopReason: resp.stop}
		_ = stream.Push(ctx, ai.EventStart{Partial: partial})
		for i, call := range resp.toolCalls {
			_ = stream.Push(ctx, ai.EventToolCallStart{ContentIndex: i, Partial: partial})
			_ = stream.Push(ctx, ai.EventToolCallDelta{ContentIndex: i, Delta: call.Args, Partial: partial})
			_ = stream.Push(ctx, ai.EventToolCallEnd{ContentIndex: i, ToolCall: call, Partial: partial})
		}
		if resp.text != "" {
			_ = stream.Push(ctx, ai.EventTextStart{ContentIndex: 0, Partial: partial})
			_ = stream.Push(ctx, ai.EventTextDelta{ContentIndex: 0, Delta: resp.text, Partial: partial})
			_ = stream.Push(ctx, ai.EventTextEnd{ContentIndex: 0, Text: resp.text, Partial: partial})
		}
		_ = stream.Push(ctx, ai.EventDone{Reason: resp.stop, Message: partial})
		stream.SetResult(partial, nil)
	}()
	return stream, nil
}

type runtimePolicyTool struct {
	name string
}

func (t runtimePolicyTool) Name() string        { return t.name }
func (t runtimePolicyTool) Description() string { return t.name }
func (t runtimePolicyTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string"},
			"command": map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		},
	}
}
func (t runtimePolicyTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	return raw, nil
}
func (t runtimePolicyTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	path := firstRuntimeStringJSONField(raw, "path")
	if path == "" {
		path = firstRuntimeStringJSONField(raw, "command")
	}
	return agent.ToolResult{Content: t.name + " ok " + path}, nil
}

func firstRuntimeStringJSONField(raw json.RawMessage, names ...string) string {
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

func TestIsPathUnder(t *testing.T) {
	assert.True(t, isPathUnder("/a/b/c/file.txt", "/a/b"))
	assert.True(t, isPathUnder("/a/b/file.txt", "/a/b"))
	assert.False(t, isPathUnder("/a/c/file.txt", "/a/b"))
	assert.False(t, isPathUnder("/x/y/file.txt", "/a/b"))
}

func TestAgentSession_AugmentInputWithPathMatchedSkills(t *testing.T) {
	s := &AgentSession{
		cfg: config.Config{Workspace: "/repo"},
		skills: []skill.Skill{{
			Name:        "docs-skill",
			WhenToUse:   "Use for docs edits.",
			Paths:       []string{"docs/**"},
			Description: "Docs",
		}},
	}

	out := s.augmentInputWithPathMatchedSkills("update docs/guide.md")
	assert.Contains(t, out, "<path_matched_skills>")
	assert.Contains(t, out, "<name>docs-skill</name>")
	assert.Contains(t, out, "update docs/guide.md")
}

func TestAgentSession_PrepareInputAutoInvokesSinglePathMatchedSkill(t *testing.T) {
	dir := t.TempDir()
	storage := session.NewJSONLStorage(filepath.Join(dir, "session.jsonl"))
	require.NoError(t, storage.Init())
	defer storage.Close()
	sess := session.New(storage)

	s := &AgentSession{
		cfg:     config.Config{Workspace: "/repo"},
		session: sess,
		agent:   agent.New(agent.Options{Session: sess}),
		skills: []skill.Skill{{
			Name:        "docs-skill",
			WhenToUse:   "Use for docs edits.",
			Paths:       []string{"docs/**"},
			Description: "Docs",
			Content:     "Follow the docs workflow.",
			FilePath:    "/repo/.claude/skills/docs-skill/SKILL.md",
			BaseDir:     "/repo/.claude/skills/docs-skill",
		}},
	}

	out := s.prepareInputForPrompt(context.Background(), "update docs/guide.md")
	assert.Contains(t, out, `<skill name="docs-skill"`)
	assert.Contains(t, out, "<skill_content>")
	assert.Contains(t, out, "User request matched this skill's paths frontmatter")
	assert.NotContains(t, out, "<path_matched_skills>")

	inv, err := sess.LastSkillInvocation(context.Background())
	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.Equal(t, "docs-skill", inv.Name)
	assert.Equal(t, []string{"docs/**"}, inv.PathPatterns)
}

func TestAgentSession_PrepareInputDoesNotAutoInvokeAmbiguousPathSkills(t *testing.T) {
	s := &AgentSession{
		cfg:   config.Config{Workspace: "/repo"},
		agent: agent.New(agent.Options{}),
		skills: []skill.Skill{
			{Name: "docs-a", Paths: []string{"docs/**"}, Content: "A"},
			{Name: "docs-b", Paths: []string{"docs/**"}, Content: "B"},
		},
	}

	out := s.prepareInputForPrompt(context.Background(), "update docs/guide.md")
	assert.Contains(t, out, "<path_matched_skills>")
	assert.Contains(t, out, "<name>docs-a</name>")
	assert.Contains(t, out, "<name>docs-b</name>")
	assert.NotContains(t, out, "<skill_content>")
}

func TestAgentSession_PathSkillDiscoveryHookAddsFollowUpOnce(t *testing.T) {
	s := &AgentSession{
		pathSkillSteered: make(map[string]bool),
		skills: []skill.Skill{{
			Name:      "docs-skill",
			WhenToUse: "Use for docs edits.",
			Paths:     []string{"docs/**"},
		}},
	}
	hook := s.pathSkillDiscoveryHook("/repo")
	call := agent.ToolCallContext{
		ToolName: "read",
		Args:     json.RawMessage(`{"path":"docs/guide.md"}`),
	}

	result, err := hook(context.Background(), call, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	require.Len(t, result.FollowUpMessages, 1)
	msg := result.FollowUpMessages[0].(ai.UserMessage)
	assert.Contains(t, msg.Content[0].Text, "<path_matched_skills>")
	assert.Contains(t, msg.Content[0].Text, "<name>docs-skill</name>")

	result, err = hook(context.Background(), call, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	assert.Empty(t, result.FollowUpMessages)
}

func TestAgentSession_PathSkillDiscoveryHookSuppressesFollowUpDuringActiveSkillPolicy(t *testing.T) {
	ag := agent.New(agent.Options{})
	ag.ActivateToolPolicy(agent.ToolPolicyActivation{
		Name:         "guizang-ppt-skill",
		SkillRoot:    "/repo/.claude/skills/guizang-ppt-skill",
		AllowedTools: []string{"read", "write"},
	})
	s := &AgentSession{
		agent:            ag,
		pathSkillSteered: make(map[string]bool),
		skills: []skill.Skill{{
			Name:      "docs-skill",
			WhenToUse: "Use for docs edits.",
			Paths:     []string{"docs/**"},
		}},
	}
	hook := s.pathSkillDiscoveryHook("/repo")
	call := agent.ToolCallContext{
		ToolName: "read",
		Args:     json.RawMessage(`{"path":"docs/guide.md"}`),
	}

	result, err := hook(context.Background(), call, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	assert.Empty(t, result.FollowUpMessages)
	assert.Empty(t, s.pathSkillSteered)
}

type fakeSkillTool struct {
	skills []skill.Skill
}

func (t *fakeSkillTool) Name() string               { return "skill" }
func (t *fakeSkillTool) Description() string        { return "fake skill tool" }
func (t *fakeSkillTool) Parameters() map[string]any { return map[string]any{} }
func (t *fakeSkillTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	return params, nil
}
func (t *fakeSkillTool) Execute(context.Context, json.RawMessage, func(agent.PartialResult)) (agent.ToolResult, error) {
	return agent.ToolResult{Content: "ok"}, nil
}
func (t *fakeSkillTool) SetSkills(skills []skill.Skill) {
	t.skills = append([]skill.Skill(nil), skills...)
}

func TestDiscoverNestedSkillDirsFindsAncestorSkillRoots(t *testing.T) {
	workspace := t.TempDir()
	nestedSkills := filepath.Join(workspace, "packages", "app", ".claude", "skills")
	require.NoError(t, os.MkdirAll(nestedSkills, 0o755))

	dirs := discoverNestedSkillDirs(workspace, []string{"packages/app/src/main.go"})

	assert.Equal(t, []string{nestedSkills}, dirs)
}

func TestToolCallPathsCanonicalizesPathToolAliases(t *testing.T) {
	cases := []struct {
		name string
		call agent.ToolCallContext
		want []string
	}{
		{
			name: "search alias",
			call: agent.ToolCallContext{ToolName: "search", Args: json.RawMessage(`{"path":"src/main.go","pattern":"main"}`)},
			want: []string{"src/main.go"},
		},
		{
			name: "glob alias",
			call: agent.ToolCallContext{ToolName: "glob", Args: json.RawMessage(`{"dir":"src","pattern":"*.go"}`)},
			want: []string{"src"},
		},
		{
			name: "case insensitive",
			call: agent.ToolCallContext{ToolName: "Read", Args: json.RawMessage(`{"file_path":"README.md"}`)},
			want: []string{"README.md"},
		},
		{
			name: "non path tool",
			call: agent.ToolCallContext{ToolName: "bash", Args: json.RawMessage(`{"command":"touch src/main.go"}`)},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, toolCallPaths(tc.call))
		})
	}
}

func TestAgentSession_PathSkillDiscoveryHookLoadsNestedSkillsAndUpdatesTool(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "packages", "app", "src", "main.go")
	skillDir := filepath.Join(workspace, "packages", "app", ".claude", "skills", "app-docs")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte("package main\n"), 0o644))
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: app-docs
description: App docs
paths:
  - src/**
argument-hint: App file path
---
Use app docs workflow.
`), 0o644))

	skillTool := &fakeSkillTool{}
	s := &AgentSession{
		cfg:              config.Config{Workspace: workspace},
		agent:            agent.New(agent.Options{Tools: []agent.Tool{skillTool}}),
		pathSkillSteered: make(map[string]bool),
		dynamicSkillDirs: make(map[string]bool),
		skillDirs:        nil,
		skills:           nil,
	}
	hook := s.pathSkillDiscoveryHook(workspace)
	call := agent.ToolCallContext{
		ToolName: "read",
		Args:     json.RawMessage(`{"path":"packages/app/src/main.go"}`),
	}

	result, err := hook(context.Background(), call, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	require.Len(t, s.skills, 1)
	assert.Equal(t, "app-docs", s.skills[0].Name)
	assert.Equal(t, skill.SourceProject, s.skills[0].Source)
	assert.ElementsMatch(t, []string{"src/**", "packages/app/src/**"}, s.skills[0].Paths)
	require.Len(t, skillTool.skills, 1)
	assert.Equal(t, "app-docs", skillTool.skills[0].Name)
	require.Len(t, result.FollowUpMessages, 1)
	msg := result.FollowUpMessages[0].(ai.UserMessage)
	assert.Contains(t, msg.Content[0].Text, "<path_matched_skills>")
	assert.Contains(t, msg.Content[0].Text, "<name>app-docs</name>")
	assert.Contains(t, msg.Content[0].Text, "<argument_hint>App file path</argument_hint>")

	result, err = hook(context.Background(), call, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	assert.Empty(t, result.FollowUpMessages)
}

func TestAgentSession_PathSkillDiscoveryHookCanonicalizesSearchAlias(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "packages", "app", "src", "main.go")
	skillDir := filepath.Join(workspace, "packages", "app", ".claude", "skills", "app-docs")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte("package main\n"), 0o644))
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: app-docs
description: App docs
paths:
  - src/**
---
Use app docs workflow.
`), 0o644))

	skillTool := &fakeSkillTool{}
	s := &AgentSession{
		cfg:              config.Config{Workspace: workspace},
		agent:            agent.New(agent.Options{Tools: []agent.Tool{skillTool}}),
		pathSkillSteered: make(map[string]bool),
		dynamicSkillDirs: make(map[string]bool),
		dynamicSkillSigs: make(map[string]string),
	}
	hook := s.pathSkillDiscoveryHook(workspace)
	call := agent.ToolCallContext{
		ToolName: "search",
		Args:     json.RawMessage(`{"path":"packages/app/src/main.go","pattern":"main"}`),
	}

	result, err := hook(context.Background(), call, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	require.Len(t, s.skills, 1)
	assert.Equal(t, "app-docs", s.skills[0].Name)
	require.Len(t, result.FollowUpMessages, 1)
	msg := result.FollowUpMessages[0].(ai.UserMessage)
	assert.Contains(t, msg.Content[0].Text, "<name>app-docs</name>")
}

func TestAgentSession_PathSkillDiscoveryHookReloadsChangedNestedSkillDir(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "packages", "app", "src", "main.go")
	skillDir := filepath.Join(workspace, "packages", "app", ".claude", "skills", "app-docs")
	skillFile := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte("package main\n"), 0o644))
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(skillFile, []byte(`---
name: app-docs
description: Old app docs
paths:
  - src/**
---
Old workflow.
`), 0o644))

	skillTool := &fakeSkillTool{}
	s := &AgentSession{
		cfg:              config.Config{Workspace: workspace},
		agent:            agent.New(agent.Options{Tools: []agent.Tool{skillTool}}),
		pathSkillSteered: make(map[string]bool),
		dynamicSkillDirs: make(map[string]bool),
		dynamicSkillSigs: make(map[string]string),
	}
	hook := s.pathSkillDiscoveryHook(workspace)
	call := agent.ToolCallContext{
		ToolName: "read",
		Args:     json.RawMessage(`{"path":"packages/app/src/main.go"}`),
	}

	_, err := hook(context.Background(), call, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	require.Len(t, s.skills, 1)
	assert.Equal(t, "Old app docs", s.skills[0].Description)

	require.NoError(t, os.WriteFile(skillFile, []byte(`---
name: app-docs
description: Updated app docs with more detail
paths:
  - src/**
---
Updated workflow.
`), 0o644))

	_, err = hook(context.Background(), call, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	require.Len(t, s.skills, 1)
	assert.Equal(t, "Updated app docs with more detail", s.skills[0].Description)
	require.Len(t, skillTool.skills, 1)
	assert.Equal(t, "Updated app docs with more detail", skillTool.skills[0].Description)
}

func TestAgentSession_PathSkillDiscoveryHookRemovesDeletedNestedSkill(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "packages", "app", "src", "main.go")
	skillDir := filepath.Join(workspace, "packages", "app", ".claude", "skills", "app-docs")
	skillFile := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte("package main\n"), 0o644))
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(skillFile, []byte(`---
name: app-docs
description: App docs
paths:
  - src/**
---
Workflow.
`), 0o644))

	skillTool := &fakeSkillTool{}
	s := &AgentSession{
		cfg:              config.Config{Workspace: workspace},
		agent:            agent.New(agent.Options{Tools: []agent.Tool{skillTool}}),
		pathSkillSteered: make(map[string]bool),
		dynamicSkillDirs: make(map[string]bool),
		dynamicSkillSigs: make(map[string]string),
	}
	hook := s.pathSkillDiscoveryHook(workspace)
	call := agent.ToolCallContext{
		ToolName: "read",
		Args:     json.RawMessage(`{"path":"packages/app/src/main.go"}`),
	}

	_, err := hook(context.Background(), call, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	require.Len(t, s.skills, 1)

	require.NoError(t, os.Remove(skillFile))

	_, err = hook(context.Background(), call, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	assert.Empty(t, s.skills)
	assert.Empty(t, skillTool.skills)
}

func TestAgentSession_AugmentInputWithPreviousSkillOnContinuation(t *testing.T) {
	dir := t.TempDir()
	storage := session.NewJSONLStorage(filepath.Join(dir, "session.jsonl"))
	require.NoError(t, storage.Init())
	defer storage.Close()
	sess := session.New(storage)
	require.NoError(t, sess.AppendSkillInvocation(context.Background(), session.SkillInvocation{
		Name:              "guizang-ppt-skill",
		Args:              "风格 A",
		BaseDir:           "/repo/.claude/skills/guizang-ppt-skill",
		Branch:            "magazine",
		AllowedSkillPaths: []string{"/repo/.claude/skills/guizang-ppt-skill/assets/template.html"},
		BlockedSkillPaths: []string{"/repo/.claude/skills/guizang-ppt-skill/assets/template-swiss.html"},
		CompactContext:    "Do not explore the skill directory.",
	}))

	s := &AgentSession{session: sess}
	out := s.augmentInputWithPreviousSkill("继续把刚才那份换成瑞士风")
	assert.Contains(t, out, "<previous_skill_context>")
	assert.Contains(t, out, "Skill: guizang-ppt-skill")
	assert.Contains(t, out, "Previous selected branch: magazine")
	assert.Contains(t, out, "Previously allowed exact skill files:")
	assert.Contains(t, out, "assets/template.html")
	assert.Contains(t, out, "Previously blocked branch-specific skill files:")
	assert.Contains(t, out, "assets/template-swiss.html")
	assert.Contains(t, out, "继续把刚才那份换成瑞士风")
}

func TestAgentSession_AugmentPreviousSkillWarnsWhenBranchUnselected(t *testing.T) {
	dir := t.TempDir()
	storage := session.NewJSONLStorage(filepath.Join(dir, "session.jsonl"))
	require.NoError(t, storage.Init())
	defer storage.Close()
	sess := session.New(storage)
	require.NoError(t, sess.AppendSkillInvocation(context.Background(), session.SkillInvocation{
		Name:              "guizang-ppt-skill",
		BaseDir:           "/repo/.claude/skills/guizang-ppt-skill",
		BlockedSkillPaths: []string{"/repo/.claude/skills/guizang-ppt-skill/assets/template-a.html"},
	}))

	s := &AgentSession{session: sess}
	out := s.augmentInputWithPreviousSkill("继续那份")
	assert.Contains(t, out, "Previously blocked branch-specific skill files:")
	assert.Contains(t, out, "assets/template-a.html")
	assert.Contains(t, out, "The branch was not selected")
	assert.Contains(t, out, "ask the user to choose the workflow branch")
}

func TestAgentSession_PrepareInputAutoReinvokesPreviousSkillContinuation(t *testing.T) {
	dir := t.TempDir()
	storage := session.NewJSONLStorage(filepath.Join(dir, "session.jsonl"))
	require.NoError(t, storage.Init())
	defer storage.Close()
	sess := session.New(storage)
	require.NoError(t, sess.AppendSkillInvocation(context.Background(), session.SkillInvocation{
		Name:    "guizang-ppt-skill",
		Args:    "风格 A 机器学习 PPT",
		BaseDir: filepath.Join(dir, "guizang-ppt-skill"),
		Branch:  "magazine",
	}))

	skillDir := filepath.Join(dir, "guizang-ppt-skill")
	for _, rel := range []string{
		"assets/template.html",
		"assets/template-swiss.html",
		"references/themes.md",
		"references/themes-swiss.md",
		"references/layouts.md",
		"references/layouts-swiss.md",
		"references/swiss-layout-lock.md",
	} {
		path := filepath.Join(skillDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(rel), 0o644))
	}
	s := &AgentSession{
		session: sess,
		agent:   agent.New(agent.Options{Session: sess}),
		skills: []skill.Skill{{
			Name:        "guizang-ppt-skill",
			Description: "Build PPT decks",
			Content:     "Create the requested deck using the selected style branch. Shared reference: `references/layouts.md`.",
			FilePath:    filepath.Join(skillDir, "SKILL.md"),
			BaseDir:     skillDir,
			Branches: []skill.SkillBranch{
				{
					Name:    "magazine",
					Aliases: []string{"风格 A", "电子杂志", "magazine"},
					Paths: []string{
						"assets/template.html",
						"references/themes.md",
						"references/layouts.md",
					},
				},
				{
					Name:    "swiss",
					Aliases: []string{"风格 B", "瑞士", "Swiss Style"},
					Paths: []string{
						"assets/template-swiss.html",
						"references/themes-swiss.md",
						"references/layouts-swiss.md",
						"references/swiss-layout-lock.md",
					},
				},
			},
		}},
	}

	out := s.prepareInputForPrompt(context.Background(), "继续把刚才那份换成瑞士风")
	assert.Contains(t, out, `<skill name="guizang-ppt-skill"`)
	assert.Contains(t, out, "<skill_content>")
	assert.Contains(t, out, "Previous invocation args:")
	assert.Contains(t, out, "Current continuation request:")
	assert.NotContains(t, out, "<previous_skill_context>")

	inv, err := sess.LastSkillInvocation(context.Background())
	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.Equal(t, "swiss", inv.Branch)
	assert.Contains(t, inv.Args, "继续把刚才那份换成瑞士风")
	assert.Contains(t, inv.AllowedSkillPaths, filepath.Join(skillDir, "assets/template-swiss.html"))
	assert.Contains(t, inv.AllowedSkillPaths, filepath.Join(skillDir, "references/themes-swiss.md"))
	assert.NotContains(t, inv.AllowedSkillPaths, filepath.Join(skillDir, "assets/template.html"))
	assert.Contains(t, inv.BlockedSkillPaths, filepath.Join(skillDir, "assets/template.html"))
	assert.Contains(t, inv.CompactContext, "Selected branch: swiss")
}

func TestAgentSession_ExplicitNamedSkillOverridesPreviousContinuation(t *testing.T) {
	dir := t.TempDir()
	storage := session.NewJSONLStorage(filepath.Join(dir, "session.jsonl"))
	require.NoError(t, storage.Init())
	defer storage.Close()
	sess := session.New(storage)
	require.NoError(t, sess.AppendSkillInvocation(context.Background(), session.SkillInvocation{
		Name:    "guizang-ppt-skill",
		Args:    "风格 A 机器学习 PPT",
		BaseDir: filepath.Join(dir, "guizang-ppt-skill"),
		Branch:  "magazine",
	}))

	guizangDir := filepath.Join(dir, "guizang-ppt-skill")
	docsDir := filepath.Join(dir, "docs-skill")
	s := &AgentSession{
		session: sess,
		agent:   agent.New(agent.Options{Session: sess}),
		skills: []skill.Skill{
			{
				Name:        "guizang-ppt-skill",
				Description: "Build PPT decks",
				Content:     "Guizang workflow.",
				FilePath:    filepath.Join(guizangDir, "SKILL.md"),
				BaseDir:     guizangDir,
			},
			{
				Name:        "docs-skill",
				Description: "Maintain docs",
				Content:     "Docs workflow.",
				FilePath:    filepath.Join(docsDir, "SKILL.md"),
				BaseDir:     docsDir,
				AllowedTools: []string{
					"read",
					"edit",
				},
			},
		},
	}

	out := s.prepareInputForPrompt(context.Background(), "继续用 docs-skill 更新说明文档")
	assert.Contains(t, out, `<skill name="docs-skill"`)
	assert.Contains(t, out, "Docs workflow.")
	assert.NotContains(t, out, `<skill name="guizang-ppt-skill"`)
	assert.NotContains(t, out, "Previous invocation args:")

	inv, err := sess.LastSkillInvocation(context.Background())
	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.Equal(t, "docs-skill", inv.Name)
	assert.Contains(t, inv.Args, "User explicitly named this skill")
	assert.ElementsMatch(t, []string{"read", "edit"}, inv.AllowedTools)
}

func TestAgentSession_ContinuationSwissPolicyBlocksOldBranchToolUse(t *testing.T) {
	dir := t.TempDir()
	storage := session.NewJSONLStorage(filepath.Join(dir, "session.jsonl"))
	require.NoError(t, storage.Init())
	defer storage.Close()
	sess := session.New(storage)
	require.NoError(t, sess.AppendSkillInvocation(context.Background(), session.SkillInvocation{
		Name:    "guizang-ppt-skill",
		Args:    "风格 A 机器学习 PPT",
		BaseDir: filepath.Join(dir, "guizang-ppt-skill"),
		Branch:  "magazine",
	}))

	skillDir := filepath.Join(dir, "guizang-ppt-skill")
	for _, rel := range []string{
		"assets/template.html",
		"assets/template-swiss.html",
		"references/themes.md",
		"references/themes-swiss.md",
		"references/layouts.md",
		"references/layouts-swiss.md",
		"references/swiss-layout-lock.md",
	} {
		path := filepath.Join(skillDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(rel), 0o644))
	}

	provider := &runtimeScriptedProvider{responses: []runtimeScriptedResponse{
		{toolCalls: []ai.ToolCall{{ID: "old", Name: "read", Args: `{"path":"assets/template.html"}`}}, stop: ai.StopReasonToolUse},
		{toolCalls: []ai.ToolCall{{ID: "new", Name: "read", Args: `{"path":"assets/template-swiss.html"}`}}, stop: ai.StopReasonToolUse},
		{text: "done", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(provider)
	ag := agent.New(agent.Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "runtime_scripted"},
		Registry: registry,
		System:   "test system",
		Session:  sess,
		Tools: []agent.Tool{
			runtimePolicyTool{name: "read"},
			runtimePolicyTool{name: "write"},
			runtimePolicyTool{name: "edit"},
			runtimePolicyTool{name: "bash"},
			runtimePolicyTool{name: "find"},
		},
		MaxTurns: 5,
	})
	s := &AgentSession{
		session: sess,
		agent:   ag,
		skills: []skill.Skill{{
			Name:        "guizang-ppt-skill",
			Description: "Build PPT decks",
			Content:     "Create the requested deck using the selected style branch.",
			FilePath:    filepath.Join(skillDir, "SKILL.md"),
			BaseDir:     skillDir,
			Branches: []skill.SkillBranch{
				{
					Name:    "magazine",
					Aliases: []string{"风格 A", "电子杂志", "magazine"},
					Paths: []string{
						"assets/template.html",
						"references/themes.md",
						"references/layouts.md",
					},
				},
				{
					Name:    "swiss",
					Aliases: []string{"风格 B", "瑞士", "Swiss Style"},
					Paths: []string{
						"assets/template-swiss.html",
						"references/themes-swiss.md",
						"references/layouts-swiss.md",
						"references/swiss-layout-lock.md",
					},
				},
			},
		}},
	}

	prepared := s.prepareInputForPrompt(context.Background(), "继续把刚才那份换成瑞士风")
	assert.Contains(t, prepared, `<skill name="guizang-ppt-skill"`)
	snapshot := s.ActiveToolPolicySnapshot()
	require.True(t, snapshot.Active)
	assert.Equal(t, "swiss", snapshot.Branch)
	assert.Contains(t, snapshot.AllowedSkillPaths, filepath.Join(skillDir, "assets/template-swiss.html"))
	assert.Contains(t, snapshot.BlockedSkillPaths, filepath.Join(skillDir, "assets/template.html"))

	_, err := ag.Prompt(context.Background(), ai.NewTextUserMessage(prepared))
	require.NoError(t, err)

	results := make(map[string]ai.ToolResultMessage)
	for _, req := range provider.requests {
		for _, msg := range req.Messages {
			if tr, ok := msg.(ai.ToolResultMessage); ok {
				results[tr.ToolCallID] = tr
			}
		}
	}
	require.Len(t, results, 2)
	assert.True(t, results["old"].IsError)
	assert.Contains(t, results["old"].Content, "outside the selected branch allowlist")
	assert.False(t, results["new"].IsError)
	assert.Contains(t, results["new"].Content, "read ok assets/template-swiss.html")

	require.NotEmpty(t, provider.requests)
	var toolNames []string
	for _, tool := range provider.requests[0].Tools {
		toolNames = append(toolNames, tool.Name)
	}
	assert.ElementsMatch(t, []string{"read", "write", "edit", "bash"}, toolNames)
}

func TestAgentSession_GuizangStyleSwitchTraceHardStopsExploration(t *testing.T) {
	dir := t.TempDir()
	storage := session.NewJSONLStorage(filepath.Join(dir, "session.jsonl"))
	require.NoError(t, storage.Init())
	defer storage.Close()
	sess := session.New(storage)

	skillDir := filepath.Join(dir, "guizang-ppt-skill")
	for _, rel := range []string{
		"assets/template.html",
		"assets/template-swiss.html",
		"references/themes.md",
		"references/themes-swiss.md",
		"references/layouts.md",
		"references/layouts-swiss.md",
		"references/swiss-layout-lock.md",
	} {
		path := filepath.Join(skillDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(rel), 0o644))
	}

	provider := &runtimeScriptedProvider{responses: []runtimeScriptedResponse{
		{toolCalls: []ai.ToolCall{{ID: "a_write", Name: "write", Args: `{"path":"deck/out.html","content":"magazine"}`}}, stop: ai.StopReasonToolUse},
		{text: "A done", stop: ai.StopReasonStop},
		{toolCalls: []ai.ToolCall{{ID: "old_asset", Name: "read", Args: `{"path":"assets/template.html"}`}}, stop: ai.StopReasonToolUse},
		{toolCalls: []ai.ToolCall{{ID: "skill_md", Name: "bash", Args: `{"command":"cp <SKILL_ROOT>/SKILL.md deck/skill.md"}`}}, stop: ai.StopReasonToolUse},
		{toolCalls: []ai.ToolCall{{ID: "late_read", Name: "read", Args: `{"path":"assets/template-swiss.html"}`}}, stop: ai.StopReasonToolUse},
		{toolCalls: []ai.ToolCall{{ID: "b_write", Name: "write", Args: `{"path":"deck/out.html","content":"swiss"}`}}, stop: ai.StopReasonToolUse},
		{text: "B done", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(provider)
	ag := agent.New(agent.Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "runtime_scripted"},
		Registry: registry,
		System:   "test system",
		Session:  sess,
		Tools: []agent.Tool{
			runtimePolicyTool{name: "read"},
			runtimePolicyTool{name: "write"},
			runtimePolicyTool{name: "edit"},
			runtimePolicyTool{name: "bash"},
			runtimePolicyTool{name: "find"},
			runtimePolicyTool{name: "grep"},
			runtimePolicyTool{name: "ls"},
		},
		MaxTurns: 8,
	})
	s := &AgentSession{
		session: sess,
		agent:   ag,
		skills: []skill.Skill{{
			Name:        "guizang-ppt-skill",
			Description: "Build PPT decks",
			Content:     "Create the requested deck using the selected style branch.",
			FilePath:    filepath.Join(skillDir, "SKILL.md"),
			BaseDir:     skillDir,
			Branches: []skill.SkillBranch{
				{
					Name:    "magazine",
					Aliases: []string{"风格 A", "电子杂志", "magazine"},
					Paths: []string{
						"assets/template.html",
						"references/themes.md",
						"references/layouts.md",
					},
				},
				{
					Name:    "swiss",
					Aliases: []string{"风格 B", "瑞士", "Swiss Style"},
					Paths: []string{
						"assets/template-swiss.html",
						"references/themes-swiss.md",
						"references/layouts-swiss.md",
						"references/swiss-layout-lock.md",
					},
				},
			},
		}},
	}

	first := s.prepareInputForPrompt(context.Background(), "使用 guizang-ppt-skill 做一份机器学习 PPT，风格 A 电子杂志")
	assert.Contains(t, first, `<skill name="guizang-ppt-skill"`)
	firstSnapshot := s.ActiveToolPolicySnapshot()
	require.True(t, firstSnapshot.Active)
	assert.Equal(t, "magazine", firstSnapshot.Branch)
	_, err := ag.Prompt(context.Background(), ai.NewTextUserMessage(first))
	require.NoError(t, err)

	second := s.prepareInputForPrompt(context.Background(), "继续把刚才那份换成瑞士风")
	assert.Contains(t, second, `<skill name="guizang-ppt-skill"`)
	secondSnapshot := s.ActiveToolPolicySnapshot()
	require.True(t, secondSnapshot.Active)
	assert.Equal(t, "swiss", secondSnapshot.Branch)
	assert.Contains(t, secondSnapshot.BlockedSkillPaths, filepath.Join(skillDir, "assets", "template.html"))
	_, err = ag.Prompt(context.Background(), ai.NewTextUserMessage(second))
	require.NoError(t, err)

	results := runtimeToolResults(provider.requests)
	require.Contains(t, results, "a_write")
	require.Contains(t, results, "old_asset")
	require.Contains(t, results, "skill_md")
	require.Contains(t, results, "late_read")
	require.Contains(t, results, "b_write")
	assert.False(t, results["a_write"].IsError)
	assert.True(t, results["old_asset"].IsError)
	assert.Contains(t, results["old_asset"].Content, "outside the selected branch allowlist")
	assert.True(t, results["skill_md"].IsError)
	assert.Contains(t, results["skill_md"].Content, "reads SKILL.md again")
	assert.Contains(t, results["skill_md"].Content, "Read/search/exploration has been terminated")
	assert.True(t, results["late_read"].IsError)
	assert.Contains(t, results["late_read"].Content, `tool "read" is not allowed`)
	assert.False(t, results["b_write"].IsError)

	require.GreaterOrEqual(t, len(provider.requests), 6)
	assert.ElementsMatch(t, []string{"read", "write", "edit", "bash"}, runtimeToolNames(provider.requests[0].Tools))
	assert.ElementsMatch(t, []string{"read", "write", "edit", "bash"}, runtimeToolNames(provider.requests[2].Tools))
	assert.ElementsMatch(t, []string{"write", "edit"}, runtimeToolNames(provider.requests[4].Tools))

	recoverySeen := false
	for _, req := range provider.requests {
		for _, msg := range req.Messages {
			if textMsg, ok := msg.(ai.UserMessage); ok {
				for _, block := range textMsg.Content {
					if strings.Contains(block.Text, "<skill_policy_recovery>") {
						recoverySeen = true
					}
				}
			}
		}
	}
	assert.True(t, recoverySeen, "expected a recovery steering message in the trace")
}

func runtimeToolResults(requests []ai.StreamRequest) map[string]ai.ToolResultMessage {
	results := make(map[string]ai.ToolResultMessage)
	for _, req := range requests {
		for _, msg := range req.Messages {
			if tr, ok := msg.(ai.ToolResultMessage); ok {
				results[tr.ToolCallID] = tr
			}
		}
	}
	return results
}

func runtimeToolNames(defs []ai.ToolDefinition) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	return names
}

func TestAgentSession_PrepareInputAutoReinvokesWhenPreviousSkillNamed(t *testing.T) {
	dir := t.TempDir()
	storage := session.NewJSONLStorage(filepath.Join(dir, "session.jsonl"))
	require.NoError(t, storage.Init())
	defer storage.Close()
	sess := session.New(storage)
	require.NoError(t, sess.AppendSkillInvocation(context.Background(), session.SkillInvocation{
		Name:    "guizang-ppt-skill",
		Args:    "风格 A 机器学习 PPT",
		BaseDir: filepath.Join(dir, "guizang-ppt-skill"),
		Branch:  "magazine",
	}))

	skillDir := filepath.Join(dir, "guizang-ppt-skill")
	s := &AgentSession{
		session: sess,
		agent:   agent.New(agent.Options{Session: sess}),
		skills: []skill.Skill{{
			Name:        "guizang-ppt-skill",
			Description: "Build PPT decks",
			Content:     "Create the requested deck.",
			FilePath:    filepath.Join(skillDir, "SKILL.md"),
			BaseDir:     skillDir,
		}},
	}

	out := s.prepareInputForPrompt(context.Background(), "用 guizang-ppt-skill 导出最终 HTML")

	assert.Contains(t, out, `<skill name="guizang-ppt-skill"`)
	assert.Contains(t, out, "<skill_content>")
	assert.NotContains(t, out, "<previous_skill_context>")
	inv, err := sess.LastSkillInvocation(context.Background())
	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.Contains(t, inv.Args, "用 guizang-ppt-skill 导出最终 HTML")
}

func TestAgentSession_DoesNotAugmentPreviousSkillForUnrelatedInput(t *testing.T) {
	dir := t.TempDir()
	storage := session.NewJSONLStorage(filepath.Join(dir, "session.jsonl"))
	require.NoError(t, storage.Init())
	defer storage.Close()
	sess := session.New(storage)
	require.NoError(t, sess.AppendSkillInvocation(context.Background(), session.SkillInvocation{Name: "deck"}))

	s := &AgentSession{session: sess}
	assert.Equal(t, "hello", s.augmentInputWithPreviousSkill("hello"))
}
