package tokenizer

import (
	"math"
	"strings"
	"sync"
	"unicode"
)

// Counter counts model input tokens for budgeting decisions.
type Counter interface {
	CountText(text string) int
}

// Backend provides token counters for one or more model families.
type Backend interface {
	Name() string
	SupportsModel(model string) bool
	ForModel(model string) Counter
}

type Registry struct {
	mu       sync.RWMutex
	backends []Backend
}

var defaultRegistry = NewRegistry(heuristicBackend{})

// NewRegistry creates a token-counter backend registry. Later backends win over
// earlier ones, so precise provider tokenizers can override the heuristic
// fallback without changing call sites.
func NewRegistry(backends ...Backend) *Registry {
	r := &Registry{}
	for _, backend := range backends {
		r.Register(backend)
	}
	return r
}

func (r *Registry) Register(backend Backend) {
	if backend == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends = append(r.backends, backend)
}

func (r *Registry) ForModel(model string) Counter {
	if r == nil {
		return heuristicCounter{family: modelFamily(model)}
	}
	r.mu.RLock()
	backends := append([]Backend(nil), r.backends...)
	r.mu.RUnlock()
	for i := len(backends) - 1; i >= 0; i-- {
		backend := backends[i]
		if !backend.SupportsModel(model) {
			continue
		}
		if counter := backend.ForModel(model); counter != nil {
			return counter
		}
	}
	return heuristicCounter{family: modelFamily(model)}
}

func (r *Registry) BackendNameForModel(model string) string {
	if r == nil {
		return heuristicBackend{}.Name()
	}
	r.mu.RLock()
	backends := append([]Backend(nil), r.backends...)
	r.mu.RUnlock()
	for i := len(backends) - 1; i >= 0; i-- {
		backend := backends[i]
		if backend.SupportsModel(model) && backend.ForModel(model) != nil {
			return backend.Name()
		}
	}
	return heuristicBackend{}.Name()
}

// RegisterBackend installs a token-counter backend in the process-wide registry.
// Backends registered later take precedence over earlier backends.
func RegisterBackend(backend Backend) {
	defaultRegistry.Register(backend)
}

// ForModel returns the best available local counter for a model name.
func ForModel(model string) Counter {
	return defaultRegistry.ForModel(model)
}

// CountText counts text tokens using the best available local counter for model.
func CountText(model, text string) int {
	return ForModel(model).CountText(text)
}

// BackendNameForModel returns the backend selected for a model. This is mainly
// useful for diagnostics and tests.
func BackendNameForModel(model string) string {
	return defaultRegistry.BackendNameForModel(model)
}

type heuristicBackend struct{}

func (heuristicBackend) Name() string {
	return "heuristic"
}

func (heuristicBackend) SupportsModel(model string) bool {
	return true
}

func (heuristicBackend) ForModel(model string) Counter {
	return heuristicCounter{family: modelFamily(model)}
}

type heuristicCounter struct {
	family string
}

func (c heuristicCounter) CountText(text string) int {
	if text == "" {
		return 0
	}
	tokens := 0
	runes := []rune(text)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case isCJK(r):
			tokens++
			i++
		case isASCIIWord(r):
			j := i + 1
			for j < len(runes) && isASCIIWord(runes[j]) {
				j++
			}
			tokens += c.countASCIIWord(runes[i:j])
			i = j
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			j := i + 1
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsNumber(runes[j])) && !isCJK(runes[j]) {
				j++
			}
			tokens += max(1, int(math.Ceil(float64(j-i)/2.0)))
			i = j
		default:
			tokens++
			i++
		}
	}
	return max(1, tokens)
}

func (c heuristicCounter) countASCIIWord(word []rune) int {
	n := len(word)
	if n == 0 {
		return 0
	}
	switch c.family {
	case "openai":
		return max(1, int(math.Ceil(float64(n)/4.0)))
	case "anthropic":
		return max(1, int(math.Ceil(float64(n)/3.8)))
	default:
		return max(1, int(math.Ceil(float64(n)/4.0)))
	}
}

func modelFamily(model string) string {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "claude"):
		return "anthropic"
	case strings.HasPrefix(lower, "gpt-"), strings.HasPrefix(lower, "o1"), strings.HasPrefix(lower, "o3"), strings.HasPrefix(lower, "o4"):
		return "openai"
	default:
		return "generic"
	}
}

func isASCIIWord(r rune) bool {
	return r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '-')
}

func isCJK(r rune) bool {
	return (r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x3040 && r <= 0x30FF) ||
		(r >= 0xAC00 && r <= 0xD7AF)
}
