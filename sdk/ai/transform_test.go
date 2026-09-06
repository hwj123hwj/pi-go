package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformMessages_ImageDowngrade(t *testing.T) {
	msgs := []Message{
		UserMessage{Content: []ContentBlock{
			{Type: "text", Text: "Hello"},
			{Type: "image", Image: &ImageBlock{URL: "http://example.com/img.png"}},
		}},
	}

	opts := DefaultTransformOptions()
	opts.NoImageSupport = true
	result := TransformMessages(msgs, opts)

	user := result[0].(UserMessage)
	assert.Equal(t, 2, len(user.Content))
	assert.Equal(t, "text", user.Content[0].Type)
	assert.Equal(t, "[Image]", user.Content[1].Text)
}

func TestTransformMessages_NoImageDowngrade(t *testing.T) {
	msgs := []Message{
		UserMessage{Content: []ContentBlock{
			{Type: "text", Text: "Hello"},
			{Type: "image", Image: &ImageBlock{URL: "http://example.com/img.png"}},
		}},
	}

	opts := DefaultTransformOptions()
	opts.NoImageSupport = false
	result := TransformMessages(msgs, opts)

	user := result[0].(UserMessage)
	assert.Equal(t, 2, len(user.Content))
	assert.Equal(t, "image", user.Content[1].Type)
}

func TestTransformMessages_NormalizeToolCallIDs(t *testing.T) {
	msgs := []Message{
		AssistantMessage{
			Text: "using tool",
			ToolCalls: []ToolCall{
				{ID: "", Name: "bash", Args: `{"command":"ls"}`},
			},
		},
	}

	opts := DefaultTransformOptions()
	opts.NormalizeToolCallIDs = true
	result := TransformMessages(msgs, opts)

	assistant := result[0].(AssistantMessage)
	assert.NotEmpty(t, assistant.ToolCalls[0].ID)
	assert.Contains(t, assistant.ToolCalls[0].ID, "generated_tc_")
}

func TestTransformMessages_MergeConsecutive(t *testing.T) {
	msgs := []Message{
		NewTextUserMessage("msg1"),
		NewTextUserMessage("msg2"),
	}

	opts := DefaultTransformOptions()
	opts.MergeConsecutiveRoles = true
	result := TransformMessages(msgs, opts)

	assert.Len(t, result, 1)
	user := result[0].(UserMessage)
	assert.Equal(t, 2, len(user.Content))
}

func TestTransformMessages_Empty(t *testing.T) {
	result := TransformMessages(nil, DefaultTransformOptions())
	assert.Nil(t, result)
}

func TestValidateMessageSequence_Valid(t *testing.T) {
	msgs := []Message{
		NewTextUserMessage("hi"),
		AssistantMessage{Text: "hello"},
		NewTextUserMessage("how are you?"),
	}
	err := ValidateMessageSequence(msgs)
	assert.NoError(t, err)
}

func TestValidateMessageSequence_ConsecutiveAssistant(t *testing.T) {
	msgs := []Message{
		NewTextUserMessage("hi"),
		AssistantMessage{Text: "hello"},
		AssistantMessage{Text: "world"},
	}
	err := ValidateMessageSequence(msgs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consecutive assistant")
}

func TestValidateMessageSequence_ToolResultFollowedByUser(t *testing.T) {
	msgs := []Message{
		NewTextUserMessage("hi"),
		AssistantMessage{Text: "let me check", ToolCalls: []ToolCall{{ID: "tc1", Name: "bash", Args: `{}`}}},
		ToolResultMessage{ToolCallID: "tc1", Content: "ok"},
		NewTextUserMessage("next question"),
	}
	err := ValidateMessageSequence(msgs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be followed by assistant")
}

func TestValidateMessageSequence_Empty(t *testing.T) {
	err := ValidateMessageSequence(nil)
	assert.NoError(t, err)
}

func TestMessagesToJSON(t *testing.T) {
	msgs := []Message{
		NewTextUserMessage("hello"),
		AssistantMessage{Text: "world"},
	}
	json, err := MessagesToJSON(msgs)
	require.NoError(t, err)
	assert.Contains(t, json, "hello")
	assert.Contains(t, json, "world")
}
