package skill

import (
	"context"
	"fmt"
	"path"
	"strings"
)

type MCPSkillResource struct {
	URI  string
	Name string
}

type MCPSkillClient interface {
	ListSkillResources(ctx context.Context) ([]MCPSkillResource, error)
	ReadSkillResource(ctx context.Context, uri string) ([]byte, error)
}

type MCPSkillSourceProvider struct {
	ProviderName string
	ServerName   string
	Client       MCPSkillClient
	Source       string
	SourceRoot   string
}

func NewMCPSkillSourceProvider(serverName string, client MCPSkillClient) MCPSkillSourceProvider {
	return MCPSkillSourceProvider{
		ProviderName: "mcp:" + strings.TrimSpace(serverName),
		ServerName:   strings.TrimSpace(serverName),
		Client:       client,
		Source:       SourcePlugin,
		SourceRoot:   "mcp://" + strings.TrimSpace(serverName),
	}
}

func (p MCPSkillSourceProvider) Name() string {
	if strings.TrimSpace(p.ProviderName) != "" {
		return p.ProviderName
	}
	if strings.TrimSpace(p.ServerName) != "" {
		return "mcp:" + strings.TrimSpace(p.ServerName)
	}
	return "mcp"
}

func (p MCPSkillSourceProvider) LoadSkills(ctx context.Context) LoadResult {
	if p.Client == nil {
		return LoadResult{Diagnostics: []Diagnostic{{
			Code:    DiagReadFailed,
			Message: "mcp skill source has no client",
			Path:    p.sourceRoot(),
		}}}
	}
	resources, err := p.Client.ListSkillResources(ctx)
	if err != nil {
		return LoadResult{Diagnostics: []Diagnostic{{
			Code:    DiagListFailed,
			Message: fmt.Sprintf("cannot list mcp skill resources: %v", err),
			Path:    p.sourceRoot(),
		}}}
	}
	result := LoadResult{Skills: make([]Skill, 0, len(resources)), Diagnostics: make([]Diagnostic, 0)}
	for _, res := range resources {
		uri := strings.TrimSpace(res.URI)
		if uri == "" {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code:    DiagReadFailed,
				Message: "mcp skill resource uri is empty",
				Path:    p.sourceRoot(),
			})
			continue
		}
		data, err := p.Client.ReadSkillResource(ctx, uri)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code:    DiagReadFailed,
				Message: fmt.Sprintf("cannot read mcp skill resource: %v", err),
				Path:    uri,
			})
			continue
		}
		sk, diags := parseSkillDocument(uri, string(data))
		result.Diagnostics = append(result.Diagnostics, diags...)
		if sk == nil {
			continue
		}
		if sk.Source == "" {
			sk.Source = p.source()
		}
		if sk.SourceRoot == "" {
			sk.SourceRoot = p.sourceRoot()
		}
		sk.FilePath = uri
		sk.BaseDir = mcpResourceBase(uri)
		result.Skills = append(result.Skills, *sk)
	}
	return result
}

func (p MCPSkillSourceProvider) source() string {
	if p.Source != "" {
		return p.Source
	}
	return SourcePlugin
}

func (p MCPSkillSourceProvider) sourceRoot() string {
	if p.SourceRoot != "" {
		return p.SourceRoot
	}
	if strings.TrimSpace(p.ServerName) != "" {
		return "mcp://" + strings.TrimSpace(p.ServerName)
	}
	return "mcp://"
}

func mcpResourceBase(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	idx := strings.LastIndex(uri, "/")
	if idx < 0 {
		return uri
	}
	prefix := uri[:idx]
	if strings.Contains(prefix, "://") {
		return prefix
	}
	return path.Dir(uri)
}
