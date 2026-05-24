package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValid(t *testing.T) {
	assert.True(t, Valid("coding"))
	assert.True(t, Valid("review"))
	assert.False(t, Valid("unknown"))
	assert.False(t, Valid(""))
}

func TestAll(t *testing.T) {
	all := All()
	assert.Equal(t, []string{"coding", "review"}, all)
}

func TestPromptFor_Coding(t *testing.T) {
	p := PromptFor(ProfileCoding)
	assert.Equal(t, "", p) // coding uses default prompt, no override
}

func TestPromptFor_Review(t *testing.T) {
	p := PromptFor(ProfileReview)
	assert.NotEmpty(t, p)
	assert.Contains(t, p, "Code Review")
	assert.Contains(t, p, "REVIEW mode")
}

func TestPromptFor_Unknown(t *testing.T) {
	p := PromptFor("unknown")
	assert.Empty(t, p) // unknown falls back to default
}

func TestPromptAppendFor_Coding(t *testing.T) {
	p := PromptAppendFor(ProfileCoding)
	assert.Empty(t, p) // no additional append for coding
}

func TestPromptAppendFor_Review(t *testing.T) {
	p := PromptAppendFor(ProfileReview)
	assert.NotEmpty(t, p)
	assert.Contains(t, p, "Review Mode Active")
}

func TestFormatList(t *testing.T) {
	list := FormatList("coding")
	assert.Contains(t, list, "coding")
	assert.Contains(t, list, "review")
	assert.Contains(t, list, "→ coding")
	assert.NotContains(t, list, "→ review")
}

func TestFormatList_NoCurrent(t *testing.T) {
	list := FormatList("")
	assert.NotContains(t, list, "→")
}
