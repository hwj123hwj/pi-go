package tools

import (
	"context"
	"testing"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/skill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillTool_InvokeExisting(t *testing.T) {
	skills := []skill.Skill{
		{Name: "research", Description: "Deep research", Content: "# Research\nDo deep research.", FilePath: "/tmp/SKILL.md", BaseDir: "/tmp"},
	}
	tool := NewSkillTool(skills)
	assert.Equal(t, "skill", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"skill":"research"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "/tmp/SKILL.md")
	assert.Contains(t, result.Content, "Follow the instructions")
	assert.Contains(t, result.Content, "research")
	require.Len(t, result.FollowUpMessages, 1)
	followUpText := followUpTextForTest(t, result.FollowUpMessages[0])
	assert.Contains(t, followUpText, "<skill_execution_contract>")
	assert.Contains(t, followUpText, "Do not list, find, grep, or otherwise explore the skill directory.")
	assert.Contains(t, followUpText, "SKILL_ROOT=/tmp")
	require.NotNil(t, result.ActivatePolicy)
	assert.Equal(t, "research", result.ActivatePolicy.Name)
	assert.Equal(t, "/tmp/SKILL.md", result.ActivatePolicy.FilePath)
	assert.Equal(t, "/tmp", result.ActivatePolicy.SkillRoot)
	assert.Contains(t, result.ActivatePolicy.CompactContext, "Skill execution contract")
}

func TestSkillTool_InvokeWithArgs(t *testing.T) {
	skills := []skill.Skill{
		{Name: "research", Description: "Deep research", Content: "# Research\nDo deep research.", FilePath: "/tmp/SKILL.md", BaseDir: "/tmp"},
	}
	tool := NewSkillTool(skills)

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"skill":"research","args":"analyze pi-go"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "/tmp/SKILL.md")
	require.NotNil(t, result.ActivatePolicy)
	assert.Equal(t, "analyze pi-go", result.ActivatePolicy.Args)
	require.Len(t, result.FollowUpMessages, 1)
	assert.Contains(t, followUpTextForTest(t, result.FollowUpMessages[0]), "<additional_instructions>")
	assert.Contains(t, followUpTextForTest(t, result.FollowUpMessages[0]), "analyze pi-go")
}

func TestSkillTool_InvokeNotFound(t *testing.T) {
	skills := []skill.Skill{
		{Name: "research", Description: "Deep research", Content: "content", FilePath: "/tmp/SKILL.md", BaseDir: "/tmp"},
	}
	tool := NewSkillTool(skills)

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"skill":"nonexistent"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "not found")
}

func TestSkillTool_EmptyName(t *testing.T) {
	tool := NewSkillTool(nil)

	_, err := tool.Validate([]byte(`{"skill":""}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestSkillTool_InvalidJSON(t *testing.T) {
	tool := NewSkillTool(nil)

	_, err := tool.Validate([]byte(`{invalid`))
	assert.Error(t, err)
}

func TestSkillTool_EmptySkills(t *testing.T) {
	tool := NewSkillTool(nil)

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"skill":"anything"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func followUpTextForTest(t *testing.T, msg ai.Message) string {
	t.Helper()
	userMsg, ok := msg.(ai.UserMessage)
	require.True(t, ok, "expected follow-up message to be ai.UserMessage")
	require.NotEmpty(t, userMsg.Content)
	return userMsg.Content[0].Text
}
