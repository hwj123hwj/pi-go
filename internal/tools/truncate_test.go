package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateOutput_ShortInput(t *testing.T) {
	input := "hello world"
	result := TruncateOutput(input, 100)
	assert.Equal(t, input, result)
}

func TestTruncateOutput_ExactFit(t *testing.T) {
	input := strings.Repeat("a", 100)
	result := TruncateOutput(input, 100)
	assert.Equal(t, input, result)
}

func TestTruncateOutput_Truncation(t *testing.T) {
	input := strings.Repeat("a", 200)
	result := TruncateOutput(input, 100)
	assert.True(t, len(result) > 100, "result should include truncation marker")
	assert.Contains(t, result, "[truncated 100 characters]")
	// Check front 80 chars preserved
	assert.True(t, strings.HasPrefix(result, strings.Repeat("a", 80)))
	// Check back 20 chars preserved (they're at the end before the trailing newline)
	assert.Contains(t, result, strings.Repeat("a", 20))
}

func TestTruncateOutput_DefaultMaxLen(t *testing.T) {
	input := strings.Repeat("x", DefaultMaxOutputLen)
	result := TruncateOutput(input, 0) // uses default
	assert.Equal(t, input, result)     // fits exactly

	input = strings.Repeat("x", DefaultMaxOutputLen+1)
	result = TruncateOutput(input, 0)
	assert.Contains(t, result, "[truncated")
}

func TestTruncateOutput_EmptyInput(t *testing.T) {
	result := TruncateOutput("", 100)
	assert.Equal(t, "", result)
}

func TestTruncateOutput_NegativeMaxLen(t *testing.T) {
	input := "hello"
	result := TruncateOutput(input, -1)
	assert.Equal(t, input, result) // uses default, fits
}
