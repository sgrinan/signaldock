package endpoint

import (
	"slices"
	"sync"

	"uuid"
)

// Store keeps endpoints in memory.
type Store struct {
	mu        sync.RWMutex
	endpoints []Endpoint
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{}
}

func (s *Store) Insert(endpoint Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.existsLocked(endpoint.URL) {
		return ErrEndpointExists
	}

	s.endpoints = append(s.endpoints, endpoint)

	return nil
}

func (s *Store) List() []Endpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	endpoints := make([]Endpoint, len(s.endpoints))
	copy(endpoints, s.endpoints)

	return endpoints
}

func (s *Store) ByID(id uuid.UUID) (Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, endpoint := range s.endpoints {
		if endpoint.ID == id {
			return endpoint, nil
		}
	}

	return Endpoint{}, ErrEndpointNotFound
}

func (s *Store) RemoveByID(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, endpoint := range s.endpoints {
		if endpoint.ID == id {
			s.endpoints = slices.Delete(s.endpoints, i, i+1)
			return nil
		}
	}

	return ErrEndpointNotFound
}

func (s *Store) UpdateLastCheck(id uuid.UUID, result CheckResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.endpoints {
		if s.endpoints[i].ID == id {
			s.endpoints[i].LastCheck = result
			return nil
		}
	}

	return ErrEndpointNotFound
}

func (s *Store) existsLocked(url string) bool {
	for _, endpoint := range s.endpoints {
		if endpoint.URL == url {
			return true
		}
	}

	return false
}
