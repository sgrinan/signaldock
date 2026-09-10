package endpoint

import (
	"sync"

	"uuid"
)

// CheckStore keeps the latest endpoint check results in memory.
type CheckStore struct {
	mu      sync.RWMutex
	results map[uuid.UUID]CheckResult
}

// NewCheckStore returns an empty CheckStore.
func NewCheckStore() *CheckStore {
	return &CheckStore{
		results: make(map[uuid.UUID]CheckResult),
	}
}

func (s *CheckStore) Set(id uuid.UUID, result CheckResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.results[id] = result
}

func (s *CheckStore) Get(id uuid.UUID) CheckResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.results[id]
}

func (s *CheckStore) Delete(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.results, id)
}
