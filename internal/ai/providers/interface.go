package providers

import (
	"context"
	"sync"

	"github.com/hwj123hwj/pi-go/internal/ai"
)

type Provider interface {
	Name() string
	Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error)
	StreamSimple(ctx context.Context, req ai.SimpleStreamRequest) (*ai.EventStream, error)
}

type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[name]
	return provider, ok
}
