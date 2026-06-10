package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultRemoteSourceMaxBytes int64 = 4 << 20

type HTTPSkillSourceProvider struct {
	ProviderName string
	Endpoint     string
	Token        string
	Headers      map[string]string
	Source       string
	SourceRoot   string
	Client       *http.Client
	MaxBytes     int64
}

func NewHTTPSkillSourceProvider(endpoint, token string) HTTPSkillSourceProvider {
	return HTTPSkillSourceProvider{
		ProviderName: "http",
		Endpoint:     endpoint,
		Token:        token,
		Source:       SourcePlugin,
		SourceRoot:   endpoint,
	}
}

func (p HTTPSkillSourceProvider) Name() string {
	if p.ProviderName != "" {
		return p.ProviderName
	}
	return "http"
}

func (p HTTPSkillSourceProvider) LoadSkills(ctx context.Context) LoadResult {
	if strings.TrimSpace(p.Endpoint) == "" {
		return LoadResult{Diagnostics: []Diagnostic{{
			Code:    DiagReadFailed,
			Message: "remote skill endpoint is empty",
		}}}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.Endpoint, nil)
	if err != nil {
		return LoadResult{Diagnostics: []Diagnostic{{
			Code:    DiagReadFailed,
			Message: fmt.Sprintf("cannot create remote skill request: %v", err),
			Path:    p.Endpoint,
		}}}
	}
	req.Header.Set("Accept", "application/json")
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}
	for key, value := range p.Headers {
		if strings.TrimSpace(key) != "" {
			req.Header.Set(key, value)
		}
	}

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return LoadResult{Diagnostics: []Diagnostic{{
			Code:    DiagReadFailed,
			Message: fmt.Sprintf("cannot fetch remote skills: %v", err),
			Path:    p.Endpoint,
		}}}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LoadResult{Diagnostics: []Diagnostic{{
			Code:    DiagReadFailed,
			Message: fmt.Sprintf("remote skill endpoint returned HTTP %d", resp.StatusCode),
			Path:    p.Endpoint,
		}}}
	}

	maxBytes := p.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultRemoteSourceMaxBytes
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return LoadResult{Diagnostics: []Diagnostic{{
			Code:    DiagReadFailed,
			Message: fmt.Sprintf("cannot read remote skill response: %v", err),
			Path:    p.Endpoint,
		}}}
	}
	if int64(len(data)) > maxBytes {
		return LoadResult{Diagnostics: []Diagnostic{{
			Code:    DiagReadFailed,
			Message: fmt.Sprintf("remote skill response exceeds %d bytes", maxBytes),
			Path:    p.Endpoint,
		}}}
	}

	skills, diags := parseRemoteSkills(data, p.source(), p.sourceRoot())
	return LoadResult{Skills: skills, Diagnostics: diags}
}

func (p HTTPSkillSourceProvider) source() string {
	if p.Source != "" {
		return p.Source
	}
	return SourcePlugin
}

func (p HTTPSkillSourceProvider) sourceRoot() string {
	if p.SourceRoot != "" {
		return p.SourceRoot
	}
	return p.Endpoint
}

type remoteSkillIndex struct {
	Skills []Skill `json:"skills"`
}

func parseRemoteSkills(data []byte, source, sourceRoot string) ([]Skill, []Diagnostic) {
	var index remoteSkillIndex
	if err := json.Unmarshal(data, &index); err != nil || index.Skills == nil {
		var direct []Skill
		if directErr := json.Unmarshal(data, &direct); directErr != nil {
			if err != nil {
				return nil, []Diagnostic{{
					Code:    DiagParseFailed,
					Message: fmt.Sprintf("cannot parse remote skill index: %v", err),
				}}
			}
			return nil, []Diagnostic{{
				Code:    DiagParseFailed,
				Message: fmt.Sprintf("cannot parse remote skill index: %v", directErr),
			}}
		}
		index.Skills = direct
	}

	out := make([]Skill, 0, len(index.Skills))
	var diags []Diagnostic
	for _, sk := range index.Skills {
		sk.Name = strings.TrimSpace(sk.Name)
		sk.Description = strings.TrimSpace(sk.Description)
		if sk.Name == "" || !validNameRegex.MatchString(sk.Name) || len(sk.Name) > maxNameLength {
			diags = append(diags, Diagnostic{
				Code:    DiagInvalidName,
				Message: fmt.Sprintf("invalid remote skill name: %q", sk.Name),
				Path:    sk.FilePath,
			})
			continue
		}
		if sk.Description == "" || len(sk.Description) > maxDescriptionLength {
			diags = append(diags, Diagnostic{
				Code:    DiagInvalidDesc,
				Message: fmt.Sprintf("invalid remote skill description for %q", sk.Name),
				Path:    sk.FilePath,
			})
			continue
		}
		if sk.Source == "" {
			sk.Source = source
		}
		if sk.SourceRoot == "" {
			sk.SourceRoot = sourceRoot
		}
		if sk.FilePath == "" {
			sk.FilePath = sourceRoot + "#" + sk.Name
		}
		if sk.BaseDir == "" {
			sk.BaseDir = sourceRoot
		}
		out = append(out, sk)
	}
	return out, diags
}
