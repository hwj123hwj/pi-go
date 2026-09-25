package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Spec 校验 ───────────────────────────────────────────────────────────────

func TestParseSpec_Valid(t *testing.T) {
	spec, err := ParseSpec([]byte(`
name: demo
timeout: 30m
max_concurrency: 8
vars:
  topic: go
steps:
  - id: a
    prompt: "p1"
  - id: b
    prompt: "p2 {{vars.topic}}"
    depends_on: [a]
`))
	require.NoError(t, err)
	assert.Equal(t, "demo", spec.Name)
	assert.Equal(t, 2, len(spec.Steps))
	assert.Equal(t, []string{"a"}, spec.stepDeps()["b"])
}

func TestParseSpec_Errors(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"missing name", "steps:\n  - id: a\n    prompt: p\n", "name is required"},
		{"no steps", "name: x\nsteps: []\n", "at least one step"},
		{"dup ids", "name: x\nsteps:\n  - id: a\n    prompt: p\n  - id: a\n    prompt: q\n", "duplicate"},
		{"bad id", "name: x\nsteps:\n  - id: 'Has Space'\n    prompt: p\n", "must match"},
		{"missing prompt", "name: x\nsteps:\n  - id: a\n", "prompt is required"},
		{"unknown dep", "name: x\nsteps:\n  - id: a\n    prompt: p\n    depends_on: [ghost]\n", "unknown dependency"},
		{"cycle", "name: x\nsteps:\n  - id: a\n    prompt: p\n    depends_on: [b]\n  - id: b\n    prompt: q\n    depends_on: [a]\n", "cycle"},
		{"bad timeout", "name: x\ntimeout: soon\nsteps:\n  - id: a\n    prompt: p\n", "invalid timeout"},
		{"confirm on foreach", "name: x\nsteps:\n  - id: a\n    prompt: p\n    foreach: list\n    confirm: true\n", "confirm is not supported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSpec([]byte(tc.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// ─── 模板渲染 ────────────────────────────────────────────────────────────────

func TestRenderPrompt(t *testing.T) {
	vars := map[string]any{"topic": "golang", "n": 2}
	outputs := map[string]StepOutput{
		"a": {Output: "AAA", Outputs: []string{"AAA"}},
	}

	out, err := renderPrompt("topic={{vars.topic}} n={{vars.n}} a={{steps.a.output}}", vars, outputs, nil, 0)
	require.NoError(t, err)
	assert.Equal(t, "topic=golang n=2 a=AAA", out)
}

func TestRenderPrompt_MissingKeyErrors(t *testing.T) {
	_, err := renderPrompt("{{vars.nope}}", map[string]any{}, nil, nil, 0)
	assert.Error(t, err)
}

func TestRenderPrompt_ForeachItem(t *testing.T) {
	out, err := renderPrompt("#{{index}}: {{item}}", nil, nil, "hello", 3)
	require.NoError(t, err)
	assert.Equal(t, "#3: hello", out)
}

// ─── foreach 解析 ────────────────────────────────────────────────────────────

func TestResolveForeach_VarList(t *testing.T) {
	vars := map[string]any{"topics": []any{"go", "rust"}}
	items, err := resolveForeach("topics", vars, nil)
	require.NoError(t, err)
	assert.Equal(t, []any{"go", "rust"}, items)
}

func TestResolveForeach_TemplateLines(t *testing.T) {
	out, _ := renderPrompt("x\ny", nil, nil, nil, 0)
	_ = out
	items, err := resolveForeach("{{vars.list}}", map[string]any{"list": "a\n\nb"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "b"}, items)
}

func TestResolveForeach_TemplateJSON(t *testing.T) {
	items, err := resolveForeach(`{{vars.payload}}`, map[string]any{"payload": `["one", "two"]`}, nil)
	require.NoError(t, err)
	assert.Equal(t, []any{"one", "two"}, items)
}

func TestResolveForeach_UnknownVar(t *testing.T) {
	_, err := resolveForeach("ghost", map[string]any{}, nil)
	assert.ErrorContains(t, err, "unknown var")
}

func TestResolveForeach_NotList(t *testing.T) {
	_, err := resolveForeach("scalar", map[string]any{"scalar": "str"}, nil)
	assert.ErrorContains(t, err, "want list")
}
