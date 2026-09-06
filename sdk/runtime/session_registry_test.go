package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionRegistry_New(t *testing.T) {
	reg := NewSessionRegistry()
	assert.NotNil(t, reg)
	assert.Empty(t, reg.List())
}

func TestSessionRegistry_Get_NotFound(t *testing.T) {
	reg := NewSessionRegistry()
	sess, ok := reg.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, sess)
}

func TestSessionRegistry_Delete_NotFound(t *testing.T) {
	reg := NewSessionRegistry()
	err := reg.Delete("nonexistent")
	assert.Error(t, err)
}

func TestSessionRegistry_CloseAll(t *testing.T) {
	reg := NewSessionRegistry()
	err := reg.CloseAll()
	assert.NoError(t, err)
}

func TestSessionRegistry_List_Empty(t *testing.T) {
	reg := NewSessionRegistry()
	list := reg.List()
	assert.Empty(t, list)
}
