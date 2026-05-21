package runtime

import (
	"context"
	"fmt"
	"sync"
)

// SessionRegistry manages session_id -> AgentSession mappings.
// This solves the routing problem in HTTP mode where each request
// must be bound to a specific session's runtime.
type SessionRegistry struct {
	mu       sync.Mutex
	sessions map[string]*AgentSession
}

// NewSessionRegistry creates a new empty registry.
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string]*AgentSession),
	}
}

// Get retrieves an AgentSession by ID.
// Returns the session and true if found, nil and false otherwise.
func (r *SessionRegistry) Get(id string) (*AgentSession, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[id]
	return s, ok
}

// Create creates a new AgentSession, registers it, and returns it.
func (r *SessionRegistry) Create(ctx context.Context, opts AgentSessionOptions, deps Dependencies) (*AgentSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure not already exists
	if _, ok := r.sessions[opts.SessionID]; ok {
		return nil, fmt.Errorf("session %q already exists in registry", opts.SessionID)
	}

	sess, err := NewAgentSession(ctx, opts, deps)
	if err != nil {
		return nil, err
	}

	r.sessions[sess.SessionID()] = sess
	return sess, nil
}

// Load loads an existing AgentSession into the registry.
// If a session with the same ID already exists, it returns the existing one.
func (r *SessionRegistry) Load(ctx context.Context, id string, opts AgentSessionOptions, deps Dependencies) (*AgentSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Return existing if already loaded
	if s, ok := r.sessions[id]; ok {
		return s, nil
	}

	opts.SessionID = id
	sess, err := NewAgentSession(ctx, opts, deps)
	if err != nil {
		return nil, err
	}

	r.sessions[sess.SessionID()] = sess
	return sess, nil
}

// Delete removes an AgentSession from the registry and closes it.
func (r *SessionRegistry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	sess, ok := r.sessions[id]
	if !ok {
		return fmt.Errorf("session %q not found in registry", id)
	}

	if err := sess.Close(); err != nil {
		return fmt.Errorf("close session %q: %w", id, err)
	}

	delete(r.sessions, id)
	return nil
}

// List returns all registered session IDs.
func (r *SessionRegistry) List() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids := make([]string, 0, len(r.sessions))
	for id := range r.sessions {
		ids = append(ids, id)
	}
	return ids
}

// CloseAll closes all registered sessions and clears the registry.
func (r *SessionRegistry) CloseAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var firstErr error
	for id, sess := range r.sessions {
		if err := sess.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(r.sessions, id)
	}
	return firstErr
}
