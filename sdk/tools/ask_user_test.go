package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAskUserTool_Basic(t *testing.T) {
	tool := NewAskUserTool()
	assert.Equal(t, "ask_user_question", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"questions":[{"question":"Which library?","header":"Library","options":[{"label":"React","description":"UI library"},{"label":"Vue","description":"Progressive framework"}]}]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "Which library?")
	assert.Contains(t, result.Content, "React")
	assert.Contains(t, result.Content, "Vue")
}

func TestAskUserTool_MultipleQuestions(t *testing.T) {
	tool := NewAskUserTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"questions":[` +
		`{"question":"Frontend framework?","header":"Frontend","options":[{"label":"React"},{"label":"Vue"}]},` +
		`{"question":"CSS approach?","header":"CSS","options":[{"label":"Tailwind"},{"label":"CSS Modules"}]}` +
		`]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "Frontend framework?")
	assert.Contains(t, result.Content, "CSS approach?")
}

func TestAskUserTool_EmptyQuestions(t *testing.T) {
	tool := NewAskUserTool()
	_, err := tool.Validate([]byte(`{"questions":[]}`))
	assert.Error(t, err)
}

func TestAskUserTool_TooManyQuestions(t *testing.T) {
	tool := NewAskUserTool()
	_, err := tool.Validate([]byte(`{"questions":[` +
		`{"question":"Q1?","options":[{"label":"A"},{"label":"B"}]},` +
		`{"question":"Q2?","options":[{"label":"A"},{"label":"B"}]},` +
		`{"question":"Q3?","options":[{"label":"A"},{"label":"B"}]},` +
		`{"question":"Q4?","options":[{"label":"A"},{"label":"B"}]},` +
		`{"question":"Q5?","options":[{"label":"A"},{"label":"B"}]}` +
		`]}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at most 4")
}

func TestAskUserTool_TooFewOptions(t *testing.T) {
	tool := NewAskUserTool()
	_, err := tool.Validate([]byte(`{"questions":[{"question":"Q?","options":[{"label":"Only one"}]}]}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "2-4 options")
}

func TestAskUserTool_DuplicateLabels(t *testing.T) {
	tool := NewAskUserTool()
	_, err := tool.Validate([]byte(`{"questions":[{"question":"Q?","options":[{"label":"Same"},{"label":"Same"}]}]}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate label")
}

func TestAskUserTool_OtherOption(t *testing.T) {
	tool := NewAskUserTool()
	_, err := tool.Validate([]byte(`{"questions":[{"question":"Q?","options":[{"label":"A"},{"label":"Other"}]}]}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Other")
}

func TestAskUserTool_DuplicateQuestions(t *testing.T) {
	tool := NewAskUserTool()
	_, err := tool.Validate([]byte(`{"questions":[` +
		`{"question":"Same question?","options":[{"label":"A"},{"label":"B"}]},` +
		`{"question":"Same question?","options":[{"label":"C"},{"label":"D"}]}` +
		`]}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate question")
}

func TestAskUserTool_AutoFixHeader(t *testing.T) {
	tool := NewAskUserTool()
	// Header should be auto-filled when missing
	validated, err := tool.Validate([]byte(`{"questions":[{"question":"Q?","options":[{"label":"A"},{"label":"B"}]}]}`))
	require.NoError(t, err)

	// The validated JSON should have a header
	var params AskUserParams
	require.NoError(t, json.Unmarshal(validated, &params))
	assert.Equal(t, "Question", params.Questions[0].Header)
}
