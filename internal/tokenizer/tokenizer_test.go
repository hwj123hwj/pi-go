package tokenizer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCountText_CJKCountsMoreThanASCIIHeuristic(t *testing.T) {
	chinese := CountText("claude-sonnet-4-5", strings.Repeat("机器学习", 20))
	ascii := CountText("claude-sonnet-4-5", strings.Repeat("machine", 20))

	assert.Greater(t, chinese, ascii)
	assert.GreaterOrEqual(t, chinese, 80)
}

func TestCountText_LongASCIIWordSplitsIntoMultipleTokens(t *testing.T) {
	tokens := CountText("gpt-4o", "supercalifragilisticexpialidocious")

	assert.Greater(t, tokens, 4)
	assert.Less(t, tokens, 16)
}

func TestCountText_PunctuationAndStructureContributeTokens(t *testing.T) {
	tokens := CountText("gpt-4o", `<skill name="deck"><description>Build PPT</description></skill>`)

	assert.Greater(t, tokens, 10)
}

func TestRegistry_UsesLastRegisteredMatchingBackend(t *testing.T) {
	registry := NewRegistry(
		heuristicBackend{},
		fixedBackend{name: "openai-official", match: "gpt-", tokens: 42},
		fixedBackend{name: "anthropic-official", match: "claude", tokens: 99},
	)

	assert.Equal(t, 42, registry.ForModel("gpt-4o").CountText("anything"))
	assert.Equal(t, "openai-official", registry.BackendNameForModel("gpt-4o"))
	assert.Equal(t, 99, registry.ForModel("claude-sonnet-4-5").CountText("anything"))
	assert.Equal(t, "anthropic-official", registry.BackendNameForModel("claude-sonnet-4-5"))
}

func TestRegistry_FallsBackToHeuristicWhenBackendDoesNotMatch(t *testing.T) {
	registry := NewRegistry(fixedBackend{name: "specific", match: "gpt-", tokens: 42})

	tokens := registry.ForModel("unknown-model").CountText("机器学习")

	assert.Equal(t, "heuristic", registry.BackendNameForModel("unknown-model"))
	assert.GreaterOrEqual(t, tokens, 4)
}

func TestRegisterBackend_OverridesDefaultRegistry(t *testing.T) {
	previous := defaultRegistry
	defaultRegistry = NewRegistry(heuristicBackend{})
	t.Cleanup(func() {
		defaultRegistry = previous
	})

	RegisterBackend(fixedBackend{name: "test-official", match: "gpt-", tokens: 7})

	assert.Equal(t, 7, CountText("gpt-4o", "anything"))
	assert.Equal(t, "test-official", BackendNameForModel("gpt-4o"))
}

type fixedBackend struct {
	name   string
	match  string
	tokens int
}

func (b fixedBackend) Name() string {
	return b.name
}

func (b fixedBackend) SupportsModel(model string) bool {
	return strings.Contains(model, b.match)
}

func (b fixedBackend) ForModel(model string) Counter {
	return fixedCounter(b.tokens)
}

type fixedCounter int

func (c fixedCounter) CountText(text string) int {
	return int(c)
}
