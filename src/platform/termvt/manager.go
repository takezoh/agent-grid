package termvt

import (
	"fmt"
	"sort"
	"sync"
)

// Manager owns a set of named Sessions. It is the multi-session multiplexer the
// server shell drives: one entry per live agent session. Safe for concurrent
// use.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{sessions: map[string]*Session{}}
}

// Create starts a session under id. It errors if id is already in use.
func (m *Manager) Create(id string, spec Spec) (*Session, error) {
	m.mu.Lock()
	if _, ok := m.sessions[id]; ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("termvt: session %q already exists", id)
	}
	m.mu.Unlock()

	sess, err := NewSession(spec)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Re-check under the lock in case of a concurrent Create with the same id.
	if _, ok := m.sessions[id]; ok {
		_ = sess.Close()
		return nil, fmt.Errorf("termvt: session %q already exists", id)
	}
	m.sessions[id] = sess
	return sess, nil
}

// Get returns the session for id.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// List returns the live session ids in sorted order.
func (m *Manager) List() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Remove closes and forgets the session for id.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("termvt: session %q not found", id)
	}
	return sess.Close()
}

// CloseAll closes every session.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	sessions := m.sessions
	m.sessions = map[string]*Session{}
	m.mu.Unlock()
	for _, s := range sessions {
		_ = s.Close()
	}
}
