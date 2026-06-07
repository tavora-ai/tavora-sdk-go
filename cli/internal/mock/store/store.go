// Package store holds the mock server's in-memory state. v0.1 only
// needs a place to remember a small set of canned objects across
// requests (the project, the capability catalog, agent sessions
// created during the process lifetime). The store also captures the
// seed snapshot at startup so /_admin/reset can restore it without
// reading any disk state.
package store

import (
	"encoding/json"
	"sync"
)

// Store is a goroutine-safe in-memory state container. Everything the
// mock returns either lives here (so it round-trips) or is generated
// on the fly by the handlers (canned responses).
type Store struct {
	mu       sync.RWMutex
	sessions map[string]map[string]any
	envs     map[string]map[string]envEntry // slug → key → entry
	seed     []byte                          // JSON snapshot captured at New
}

type envEntry struct {
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

// New returns an empty Store and captures its (empty) state as the
// reset snapshot. Callers that pre-populate state should call
// Snapshot+SetSeed afterwards, but for v0.1 the default empty state
// is fine — handlers fabricate canned values regardless.
func New() *Store {
	s := &Store{
		sessions: make(map[string]map[string]any),
		envs:     make(map[string]map[string]envEntry),
	}
	s.seed, _ = s.snapshot()
	return s
}

// PutSession remembers a session by id so a follow-up GET returns the
// same object the CLI's POST just created.
func (s *Store) PutSession(id string, session map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = session
}

// GetSession returns a previously remembered session, or nil.
func (s *Store) GetSession(id string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

// PutEnv upserts one (slug, key) entry.
func (s *Store) PutEnv(slug, key, value string, isSecret bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.envs[slug]
	if !ok {
		m = make(map[string]envEntry)
		s.envs[slug] = m
	}
	m[key] = envEntry{Value: value, IsSecret: isSecret}
}

// GetEnv returns one entry's plaintext value, or ("", false, false).
func (s *Store) GetEnv(slug, key string) (string, bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.envs[slug]
	if !ok {
		return "", false, false
	}
	e, ok := m[key]
	if !ok {
		return "", false, false
	}
	return e.Value, e.IsSecret, true
}

// ListEnv returns the keys + redaction flags for a slug.
func (s *Store) ListEnv(slug string) []EnvKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.envs[slug]
	out := make([]EnvKey, 0, len(m))
	for k, v := range m {
		out = append(out, EnvKey{Key: k, IsSecret: v.IsSecret})
	}
	return out
}

// DeleteEnv removes one entry. Returns true if anything was removed.
func (s *Store) DeleteEnv(slug, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.envs[slug]
	if !ok {
		return false
	}
	if _, had := m[key]; !had {
		return false
	}
	delete(m, key)
	return true
}

// EnvKey is the redacted shape ListEnv returns.
type EnvKey struct {
	Key      string
	IsSecret bool
}

// Snapshot returns a JSON dump of current state.
func (s *Store) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot()
}

// Reset restores the seed snapshot captured at New().
func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var dump struct {
		Sessions map[string]map[string]any  `json:"sessions"`
		Envs     map[string]map[string]envEntry `json:"envs"`
	}
	if err := json.Unmarshal(s.seed, &dump); err != nil {
		return err
	}
	if dump.Sessions == nil {
		dump.Sessions = map[string]map[string]any{}
	}
	if dump.Envs == nil {
		dump.Envs = map[string]map[string]envEntry{}
	}
	s.sessions = dump.Sessions
	s.envs = dump.Envs
	return nil
}

func (s *Store) snapshot() ([]byte, error) {
	return json.Marshal(struct {
		Sessions map[string]map[string]any      `json:"sessions"`
		Envs     map[string]map[string]envEntry `json:"envs"`
	}{
		Sessions: s.sessions,
		Envs:     s.envs,
	})
}
