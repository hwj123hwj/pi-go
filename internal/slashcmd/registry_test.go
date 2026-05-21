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
	name, args := parseSlashCommand("/compact reason text")
	assert.Equal(t, "compact", name)
	assert.Equal(t, "reason text", args)

	name, args = parseSlashCommand("/help")
	assert.Equal(t, "help", name)
	assert.Equal(t, "", args)

	name, args = parseSlashCommand("  /test  arg1 arg2  ")
	assert.Equal(t, "test", name)
	assert.Equal(t, "arg1 arg2", args)
}

func TestRegisterBuiltins(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	assert.Contains(t, reg.Names(), "help")
	assert.Contains(t, reg.Names(), "compact")
	assert.Contains(t, reg.Names(), "sessions")
	assert.Contains(t, reg.Names(), "session")
	assert.Contains(t, reg.Names(), "branch")
	assert.Contains(t, reg.Names(), "new")
	assert.Contains(t, reg.Names(), "tools")
	assert.Contains(t, reg.Names(), "model")
}

func TestBuiltin_Help(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	output, err := reg.Execute(Context{}, "/help")
	require.NoError(t, err)
	assert.Contains(t, output, "Available commands")
}

func TestBuiltin_Session_NoSession(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	output, err := reg.Execute(Context{}, "/session")
	require.NoError(t, err)
	assert.Equal(t, "no active session", output)
}

func TestBuiltin_Model_NoSession(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	output, err := reg.Execute(Context{}, "/model")
	require.NoError(t, err)
	assert.Equal(t, "no active session", output)
}

func TestBuiltin_Tools(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	output, err := reg.Execute(Context{}, "/tools")
	require.NoError(t, err)
	assert.Contains(t, output, "bash")
	assert.Contains(t, output, "read")
}
