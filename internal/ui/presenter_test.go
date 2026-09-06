package ui

import (
	"bytes"
	"testing"

	"github.com/hwj123hwj/pi-go/sdk/agent"
	"github.com/stretchr/testify/assert"
)

func TestPresent_TextDelta(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	p.Present(agent.AgentStreamEvent{
		Type:      agent.StreamEventTextDelta,
		TextDelta: "hello",
	})

	assert.Equal(t, "hello", buf.String())
}

func TestPresent_ToolStart(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	p.Present(agent.AgentStreamEvent{
		Type:     agent.StreamEventToolStart,
		ToolName: "bash",
	})

	assert.Contains(t, buf.String(), "▶ bash")
}

func TestPresent_ToolUpdate(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	p.Present(agent.AgentStreamEvent{
		Type:     agent.StreamEventToolUpdate,
		ToolName: "bash",
	})

	assert.Equal(t, ".", buf.String())
}

func TestPresent_ToolEnd_Success(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	p.Present(agent.AgentStreamEvent{
		Type:       agent.StreamEventToolEnd,
		ToolName:   "read",
		ToolResult: "file contents here",
		IsError:    false,
	})

	output := buf.String()
	assert.Contains(t, output, "✓")
	assert.Contains(t, output, "file contents here")
}

func TestPresent_ToolEnd_SuccessEmptyResult(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	p.Present(agent.AgentStreamEvent{
		Type:       agent.StreamEventToolEnd,
		ToolName:   "bash",
		ToolResult: "",
		IsError:    false,
	})

	output := buf.String()
	assert.Contains(t, output, "✓")
	assert.NotContains(t, output, "→")
}

func TestPresent_ToolEnd_Error(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	p.Present(agent.AgentStreamEvent{
		Type:       agent.StreamEventToolEnd,
		ToolName:   "bash",
		ToolResult: "command failed with exit code 1",
		IsError:    true,
	})

	output := buf.String()
	assert.Contains(t, output, "✗")
	assert.Contains(t, output, "error:")
	assert.Contains(t, output, "command failed")
}

func TestPresent_ToolEnd_LongResultTruncated(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	longResult := ""
	for i := 0; i < 200; i++ {
		longResult += "x"
	}

	p.Present(agent.AgentStreamEvent{
		Type:       agent.StreamEventToolEnd,
		ToolName:   "read",
		ToolResult: longResult,
		IsError:    false,
	})

	output := buf.String()
	assert.Contains(t, output, "...")
	// Should not contain full 200 chars
	assert.Less(t, len(output), 200+50) // plus overhead chars
}

func TestPresent_Compacted_WithCounts(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	p.Present(agent.AgentStreamEvent{
		Type:        agent.StreamEventCompacted,
		Summary:     "some summary",
		TrimmedFrom: 20,
		TrimmedTo:   5,
	})

	output := buf.String()
	assert.Contains(t, output, "20 → 5")
}

func TestPresent_Compacted_WithoutCounts(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	p.Present(agent.AgentStreamEvent{
		Type:    agent.StreamEventCompacted,
		Summary: "some summary",
	})

	output := buf.String()
	assert.Contains(t, output, "context compacted")
}

func TestPresent_Done(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	p.Present(agent.AgentStreamEvent{
		Type: agent.StreamEventDone,
	})

	assert.Equal(t, "\n", buf.String())
}

func TestPresent_Error(t *testing.T) {
	var buf bytes.Buffer
	p := NewPresenter(&buf)

	p.Present(agent.AgentStreamEvent{
		Type:  agent.StreamEventError,
		Error: "something went wrong",
	})

	output := buf.String()
	assert.Contains(t, output, "error:")
	assert.Contains(t, output, "something went wrong")
}

// ─── Format helpers ──────────────────────────────────────────────────────────

func TestFormatSessionStatus(t *testing.T) {
	status := FormatSessionStatus("sess_123", "anthropic", "claude-3.5-sonnet", "coding", "/home/user/project")
	assert.Contains(t, status, "sess_123")
	assert.Contains(t, status, "claude-3.5-sonnet")
	assert.Contains(t, status, "coding")
	assert.Contains(t, status, "project")
}

func TestFormatTimestamp_Zero(t *testing.T) {
	result := FormatTimestamp(0)
	assert.Equal(t, "n/a", result)
}

// ─── convertEvent tests ───────────────────────────────────────────────────────

func TestConvertEvent_TextDelta(t *testing.T) {
	de := convertEvent(agent.AgentStreamEvent{
		Type:      agent.StreamEventTextDelta,
		TextDelta: "hello world",
	})
	assert.Equal(t, DisplayTextDelta, de.Type)
	assert.Equal(t, "hello world", de.Content)
}

func TestConvertEvent_ToolEnd_Truncation(t *testing.T) {
	longResult := ""
	for i := 0; i < 200; i++ {
		longResult += "a"
	}

	de := convertEvent(agent.AgentStreamEvent{
		Type:       agent.StreamEventToolEnd,
		ToolResult: longResult,
	})
	assert.Equal(t, DisplayToolEnd, de.Type)
	assert.True(t, len(de.Content) < 200)
	assert.Contains(t, de.Content, "...")
}

func TestConvertEvent_Error(t *testing.T) {
	de := convertEvent(agent.AgentStreamEvent{
		Type:  agent.StreamEventError,
		Error: "timeout",
	})
	assert.Equal(t, DisplayError, de.Type)
	assert.Equal(t, "timeout", de.Content)
	assert.True(t, de.IsError)
}
