package slashcmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_New(t *testing.T) {
	reg := NewRegistry()
	assert.NotNil(t, reg)
	assert.Empty(t, reg.Names())
}

func TestRegistry_Register(t *testing.T) {
	reg := NewRegistry()
	reg.Register(Command{
		Name:        "test",
		Description: "test command",
		Handler: func(ctx Context, args string) (string, error) {
			return "result: " + args, nil
		},
	})
	assert.Contains(t, reg.Names(), "test")
}

func TestRegistry_Execute(t *testing.T) {
	reg := NewRegistry()
	reg.Register(Command{
		Name:        "echo",
		Description: "echo args",
		Handler: func(ctx Context, args string) (string, error) {
			return "echo: " + args, nil
		},
	})

	output, err := reg.Execute(Context{}, "/echo hello world")
	require.NoError(t, err)
	assert.Equal(t, "echo: hello world", output)
}

func TestRegistry_Execute_Unknown(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Execute(Context{}, "/unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestRegistry_Help(t *testing.T) {
	reg := NewRegistry()
	reg.Register(Command{Name: "foo", Description: "foo command"})
	reg.Register(Command{Name: "bar", Description: "bar command"})

	help := reg.Help()
	assert.Contains(t, help, "foo")
	assert.Contains(t, help, "bar")
	assert.Contains(t, help, "Available commands")
}

func TestIsSlashCommand(t *testing.T) {
	assert.True(t, IsSlashCommand("/help"))
	assert.True(t, IsSlashCommand("  /help"))
	assert.False(t, IsSlashCommand("help"))
	assert.False(t, IsSlashCommand(""))
}

func TestParseSlashCommand(t *testing.T) {
	name, args := ParseSlashCommand("/compact reason text")
	assert.Equal(t, "compact", name)
	assert.Equal(t, "reason text", args)

	name, args = ParseSlashCommand("/help")
	assert.Equal(t, "help", name)
	assert.Equal(t, "", args)

	name, args = ParseSlashCommand("  /test  arg1 arg2  ")
	assert.Equal(t, "test", name)
	assert.Equal(t, "arg1 arg2", args)
}
