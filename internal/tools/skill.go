package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/skill"
)

// SkillTool lets the LLM invoke a skill by name to load specialized instructions.
type SkillTool struct {
	mu     sync.RWMutex
	skills []skill.Skill
}

// SkillParams holds the deserialized parameters for SkillTool.
type SkillParams struct {
	Skill string `json:"skill"`
	Args  string `json:"args,omitempty"`
}

// NewSkillTool creates a SkillTool with the given skills.
func NewSkillTool(skills []skill.Skill) *SkillTool {
	return &SkillTool{skills: append([]skill.Skill(nil), skills...)}
}

func (t *SkillTool) SetSkills(skills []skill.Skill) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.skills = append([]skill.Skill(nil), skills...)
}

func (t *SkillTool) Name() string { return "skill" }
func (t *SkillTool) Description() string {
	return "Execute a skill within the main conversation. " +
		"When users ask you to perform tasks, check if any of the available skills match. " +
		"Skills provide specialized capabilities and domain knowledge. " +
		"When a skill matches the user's request, this is a BLOCKING REQUIREMENT: " +
		"invoke the relevant Skill tool BEFORE generating any other response about the task. " +
		"Available skills are listed in the system prompt under <available_skills>."
}
func (t *SkillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"skill": map[string]any{
				"type":        "string",
				"description": "Name of the skill to invoke.",
			},
			"args": map[string]any{
				"type":        "string",
				"description": "Optional arguments to pass to the skill.",
			},
		},
		"required": []string{"skill"},
	}
}

func (t *SkillTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	var p SkillParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if p.Skill == "" {
		return nil, fmt.Errorf("skill name is required")
	}
	return json.Marshal(p)
}

func (t *SkillTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p SkillParams
	if err := json.Unmarshal(params, &p); err != nil {
		return agent.ToolResult{}, err
	}

	t.mu.RLock()
	skills := append([]skill.Skill(nil), t.skills...)
	t.mu.RUnlock()

	s := skill.FindByName(skills, p.Skill)
	if s == nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("Skill %q not found. Available skills are listed in the system prompt under <available_skills>.", p.Skill),
			IsError: true,
		}, nil
	}

	// Return a brief tool result + inject the full skill content as a user message.
	// This matches cc-haha behavior: the skill content becomes a user message
	// that the model treats as instructions, not as reference material.
	skillContent := skill.FormatInvocation(*s, p.Args)

	return agent.ToolResult{
		Content:        fmt.Sprintf("Skill %q loaded from %s. Follow the instructions in the next message.", s.Name, s.FilePath),
		ActivatePolicy: ptr(skill.BuildExecutionPolicy(*s, p.Args)),
		FollowUpMessages: []ai.Message{
			ai.NewTextUserMessage(skillContent),
		},
	}, nil
}

func ptr[T any](v T) *T {
	return &v
}
