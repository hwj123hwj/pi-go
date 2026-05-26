package coding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodingSessionExt_DefaultProfile(t *testing.T) {
	ext := NewCodingSessionExt(nil)
	assert.Equal(t, "coding", ext.Profile())
}

func TestCodingSessionExt_SwitchProfile_Valid(t *testing.T) {
	rebuildCalled := false
	ext := NewCodingSessionExt(func() error {
		rebuildCalled = true
		return nil
	})
	err := ext.SwitchProfile(context.Background(), "review")
	require.NoError(t, err)
	assert.Equal(t, "review", ext.Profile())
	assert.True(t, rebuildCalled)
}

func TestCodingSessionExt_SwitchProfile_Invalid(t *testing.T) {
	ext := NewCodingSessionExt(nil)
	err := ext.SwitchProfile(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown profile")
	assert.Equal(t, "coding", ext.Profile()) // unchanged
}

func TestCodingSessionExt_Goal(t *testing.T) {
	rebuildCalled := false
	ext := NewCodingSessionExt(func() error {
		rebuildCalled = true
		return nil
	})
	assert.Equal(t, "", ext.Goal())

	ext.SetGoal("fix auth bug")
	assert.Equal(t, "fix auth bug", ext.Goal())
	assert.True(t, rebuildCalled)

	ext.ClearGoal()
	assert.Equal(t, "", ext.Goal())
}

func TestCodingSessionExt_SetRebuild(t *testing.T) {
	ext := NewCodingSessionExt(nil)
	assert.Nil(t, ext.rebuild) // not directly accessible but behavior test

	rebuildCalled := false
	ext.SetRebuild(func() error {
		rebuildCalled = true
		return nil
	})
	ext.SetGoal("test goal")
	assert.True(t, rebuildCalled)
}
