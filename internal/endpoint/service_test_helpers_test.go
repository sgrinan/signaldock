package endpoint

import (
	"net/netip"
	"net/url"
	"sync"

	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

type fakeRepository struct {
	mu        sync.RWMutex
	endpoints []Endpoint
}

type fakeCheckStore struct {
	mu      sync.RWMutex
	results map[uuid.UUID]CheckResult
}

func newTestService() (*Service, *fakeRepository, *fakeCheckStore) {
	repository := &fakeRepository{}
	checks := &fakeCheckStore{
		results: make(map[uuid.UUID]CheckResult),
	}

	s := NewService(repository, checks)

	ips := []netip.Addr{
		netip.MustParseAddr("203.0.113.10"),
	}

	s.validateHost = func(string) ([]netip.Addr, error) {
		return ips, nil
	}

	s.probeHTTP = func(*url.URL, []netip.Addr) (probe.HTTPResult, error) {
		return probe.HTTPResult{
			StatusCode: 200,
			Responded:  true,
		}, nil
	}

	s.probeTLS = func(*url.URL, []netip.Addr) (probe.TLSResult, error) {
		return probe.TLSResult{
			Enabled: true,
			Valid:   true,
		}, nil
	}

	return s, repository, checks
}

func (r *fakeRepository) Insert(endpoint Endpoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.endpoints {
		if existing.URL == endpoint.URL {
			return ErrEndpointExists
		}
	}

	r.endpoints = append(r.endpoints, Endpoint{
		ID:  endpoint.ID,
		URL: endpoint.URL,
	})

	return nil
}

func (r *fakeRepository) List() ([]Endpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	endpoints := make([]Endpoint, len(r.endpoints))
	copy(endpoints, r.endpoints)

	return endpoints, nil
}

func (r *fakeRepository) ByID(id uuid.UUID) (Endpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, endpoint := range r.endpoints {
		if endpoint.ID == id {
			return endpoint, nil
		}
	}

	return Endpoint{}, ErrEndpointNotFound
}

func (r *fakeRepository) RemoveByID(id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for index, endpoint := range r.endpoints {
		if endpoint.ID == id {
			r.endpoints = append(r.endpoints[:index], r.endpoints[index+1:]...)

			return nil
		}
	}

	return ErrEndpointNotFound
}

func (s *fakeCheckStore) Set(id uuid.UUID, result CheckResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.results[id] = result
}

func (s *fakeCheckStore) Get(id uuid.UUID) CheckResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.results[id]
}

func (s *fakeCheckStore) Delete(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.results, id)
}
