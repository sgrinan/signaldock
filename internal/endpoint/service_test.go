package endpoint

import (
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

// Service.Add

func TestService_Add(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s, store := newTestService()

		got, err := s.Add("HTTPS://EXAMPLE.COM:443")
		if err != nil {
			t.Fatalf("Add() returned unexpected error: %v", err)
		}

		if got.ID == uuid.Nil() {
			t.Errorf("Add().ID = %v, want non-zero UUID", got.ID)
		}

		if got.URL != "https://example.com/" {
			t.Errorf("Add().URL = %q, want %q", got.URL, "https://example.com/")
		}

		if !got.LastCheck.HTTP.Responded {
			t.Error("Add().LastCheck.HTTP.Responded = false, want true")
		}

		if !got.LastCheck.TLS.Valid {
			t.Error("Add().LastCheck.TLS.Valid = false, want true")
		}

		stored, err := store.ByID(got.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", got.ID, err)
		}

		if stored != got {
			t.Errorf("ByID(%v) = %+v, want %+v", got.ID, stored, got)
		}
	})

	t.Run("invalid_url", func(t *testing.T) {
		s, store := newTestService()

		const rawURL = "ftp://example.com"

		got, err := s.Add(rawURL)

		if !errors.Is(err, ErrUnsupportedScheme) {
			t.Errorf("Add(%q) error = %v, want %v", rawURL, err, ErrUnsupportedScheme)
		}

		if got != (Endpoint{}) {
			t.Errorf("Add(%q) = %+v, want zero Endpoint", rawURL, got)
		}

		endpoints, err := store.List()
		if err != nil {
			t.Fatalf("List() returned unexpected error: %v", err)
		}

		if got := len(endpoints); got != 0 {
			t.Errorf("len(List()) = %d, want 0", got)
		}
	})

	t.Run("unsafe_host", func(t *testing.T) {
		s, store := newTestService()

		s.validateHost = func(string) ([]netip.Addr, error) {
			return nil, probe.ErrUnsafeHost
		}

		const rawURL = "https://example.com"

		got, err := s.Add(rawURL)

		if !errors.Is(err, ErrUnsafeHost) {
			t.Errorf("Add(%q) error = %v, want %v", rawURL, err, ErrUnsafeHost)
		}

		if got != (Endpoint{}) {
			t.Errorf("Add(%q) = %+v, want zero Endpoint", rawURL, got)
		}

		endpoints, err := store.List()
		if err != nil {
			t.Fatalf("List() returned unexpected error: %v", err)
		}

		if got := len(endpoints); got != 0 {
			t.Errorf("len(List()) = %d, want 0", got)
		}
	})

	t.Run("host_check_error_stores_failed_result", func(t *testing.T) {
		s, store := newTestService()

		s.validateHost = func(string) ([]netip.Addr, error) {
			return nil, probe.ErrNoResolvedAddresses
		}

		const rawURL = "https://example.com"

		got, err := s.Add(rawURL)

		var checkErr CheckError
		if !errors.As(err, &checkErr) {
			t.Fatalf("Add(%q) error = %v, want CheckError", rawURL, err)
		}

		if !errors.Is(err, probe.ErrNoResolvedAddresses) {
			t.Errorf("Add(%q) error = %v, want %v", rawURL, err, probe.ErrNoResolvedAddresses)
		}

		if got.ID == uuid.Nil() {
			t.Errorf("Add(%q).ID = %v, want non-zero UUID", rawURL, got.ID)
		}

		if got.LastCheck.HTTP.Responded {
			t.Errorf("Add(%q).LastCheck.HTTP.Responded = true, want false", rawURL)
		}

		if got.LastCheck.HTTP.CheckedAt.IsZero() {
			t.Errorf("Add(%q).LastCheck.HTTP.CheckedAt is zero, want check time", rawURL)
		}

		if !got.LastCheck.TLS.Enabled {
			t.Errorf("Add(%q).LastCheck.TLS.Enabled = false, want true", rawURL)
		}

		stored, err := store.ByID(got.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", got.ID, err)
		}

		if stored != got {
			t.Errorf("ByID(%v) = %+v, want %+v", got.ID, stored, got)
		}
	})

	t.Run("check_error_stores_result", func(t *testing.T) {
		s, store := newTestService()

		httpErr := errors.New("HTTP failed")

		s.probeHTTP = func(*url.URL, []netip.Addr) (probe.HTTPResult, error) {
			return probe.HTTPResult{
				Responded: false,
			}, httpErr
		}

		const rawURL = "https://example.com"

		got, err := s.Add(rawURL)

		var checkErr CheckError
		if !errors.As(err, &checkErr) {
			t.Fatalf("Add(%q) error = %v, want CheckError", rawURL, err)
		}

		if !errors.Is(err, httpErr) {
			t.Errorf("errors.Is(Add(%q) error, httpErr) = false, want true", rawURL)
		}

		stored, err := store.ByID(got.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", got.ID, err)
		}

		if stored != got {
			t.Errorf("ByID(%v) = %+v, want %+v", got.ID, stored, got)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		s, store := newTestService()

		const rawURL = "https://example.com"

		first, err := s.Add(rawURL)
		if err != nil {
			t.Fatalf("first Add(%q) returned unexpected error: %v", rawURL, err)
		}

		_, err = s.Add(rawURL)
		if !errors.Is(err, ErrEndpointExists) {
			t.Errorf("second Add(%q) error = %v, want %v", rawURL, err, ErrEndpointExists)
		}

		endpoints, err := store.List()
		if err != nil {
			t.Fatalf("List() returned unexpected error: %v", err)
		}

		if got := len(endpoints); got != 1 {
			t.Errorf("len(List()) = %d, want 1", got)
		}

		stored, err := store.ByID(first.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", first.ID, err)
		}

		if stored != first {
			t.Errorf("ByID(%v) = %+v, want %+v", first.ID, stored, first)
		}
	})
}

func TestService_AddConcurrentDuplicate(t *testing.T) {
	s, store := newTestService()

	const goroutines = 10

	var httpCalls atomic.Int32
	var tlsCalls atomic.Int32

	s.probeHTTP = func(*url.URL, []netip.Addr) (probe.HTTPResult, error) {
		httpCalls.Add(1)

		time.Sleep(50 * time.Millisecond)

		return probe.HTTPResult{
			StatusCode: http.StatusOK,
			Responded:  true,
		}, nil
	}

	s.probeTLS = func(*url.URL, []netip.Addr) (probe.TLSResult, error) {
		tlsCalls.Add(1)

		return probe.TLSResult{
			Enabled: true,
			Valid:   true,
		}, nil
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)

	errs := make(chan error, goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			_, err := s.Add("https://example.com")
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	successes := 0
	duplicates := 0

	for err := range errs {
		switch {
		case err == nil:
			successes++

		case errors.Is(err, ErrEndpointExists):
			duplicates++

		default:
			t.Errorf("Add() returned unexpected error: %v", err)
		}
	}

	if got, want := successes, 1; got != want {
		t.Errorf("successful Add() calls = %d, want %d", got, want)
	}

	if got, want := duplicates, goroutines-1; got != want {
		t.Errorf("duplicate Add() calls = %d, want %d", got, want)
	}

	endpoints, err := store.List()
	if err != nil {
		t.Fatalf("List() returned unexpected error: %v", err)
	}

	if got, want := len(endpoints), 1; got != want {
		t.Errorf("len(List()) = %d, want %d", got, want)
	}

	if got, want := httpCalls.Load(), int32(1); got != want {
		t.Errorf("HTTP probe calls = %d, want %d", got, want)
	}

	if got, want := tlsCalls.Load(), int32(1); got != want {
		t.Errorf("TLS probe calls = %d, want %d", got, want)
	}
}

// Service.Refresh

func TestService_Refresh(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s, store := newTestService()

		ep := testEndpoint("https://example.com/")

		if err := store.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
		}

		want := CheckResult{
			HTTP: probe.HTTPResult{
				StatusCode: 204,
				Responded:  true,
			},
			TLS: probe.TLSResult{
				Enabled: true,
				Valid:   true,
			},
		}

		s.probeHTTP = func(*url.URL, []netip.Addr) (probe.HTTPResult, error) {
			return want.HTTP, nil
		}

		s.probeTLS = func(*url.URL, []netip.Addr) (probe.TLSResult, error) {
			return want.TLS, nil
		}

		got, err := s.Refresh(ep.ID)
		if err != nil {
			t.Fatalf("Refresh(%v) returned unexpected error: %v", ep.ID, err)
		}

		if got != want {
			t.Errorf("Refresh(%v) = %+v, want %+v", ep.ID, got, want)
		}

		stored, err := store.ByID(ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if stored.LastCheck != want {
			t.Errorf("ByID(%v).LastCheck = %+v, want %+v", ep.ID, stored.LastCheck, want)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		s, _ := newTestService()
		id := uuid.NewV7()

		_, err := s.Refresh(id)

		if !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("Refresh(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
		}
	})

	t.Run("host_check_error_updates_result", func(t *testing.T) {
		s, store := newTestService()

		ep := testEndpoint("https://example.com/")
		ep.LastCheck = CheckResult{
			HTTP: probe.HTTPResult{
				StatusCode: 200,
				Responded:  true,
				CheckedAt:  time.Now().Add(-time.Minute),
			},
			TLS: probe.TLSResult{
				Enabled: true,
				Valid:   true,
			},
		}

		if err := store.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
		}

		s.validateHost = func(string) ([]netip.Addr, error) {
			return nil, probe.ErrNoResolvedAddresses
		}

		got, err := s.Refresh(ep.ID)

		var checkErr CheckError
		if !errors.As(err, &checkErr) {
			t.Fatalf("Refresh(%v) error = %v, want CheckError", ep.ID, err)
		}

		if !errors.Is(err, probe.ErrNoResolvedAddresses) {
			t.Errorf("Refresh(%v) error = %v, want %v", ep.ID, err, probe.ErrNoResolvedAddresses)
		}

		if got.HTTP.Responded {
			t.Errorf("Refresh(%v).HTTP.Responded = true, want false", ep.ID)
		}

		if got.HTTP.CheckedAt.IsZero() {
			t.Errorf("Refresh(%v).HTTP.CheckedAt is zero, want check time", ep.ID)
		}

		if !got.TLS.Enabled {
			t.Errorf("Refresh(%v).TLS.Enabled = false, want true", ep.ID)
		}

		stored, err := store.ByID(ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if stored.LastCheck != got {
			t.Errorf("ByID(%v).LastCheck = %+v, want %+v", ep.ID, stored.LastCheck, got)
		}
	})

	t.Run("check_error_updates_result", func(t *testing.T) {
		s, store := newTestService()

		ep := testEndpoint("https://example.com/")

		if err := store.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
		}

		httpErr := errors.New("HTTP failed")

		wantHTTP := probe.HTTPResult{
			Responded: false,
		}

		s.probeHTTP = func(*url.URL, []netip.Addr) (probe.HTTPResult, error) {
			return wantHTTP, httpErr
		}

		got, err := s.Refresh(ep.ID)

		if _, ok := errors.AsType[CheckError](err); !ok {
			t.Fatalf("Refresh(%v) error = %v, want CheckError", ep.ID, err)
		}

		if got.HTTP != wantHTTP {
			t.Errorf("Refresh(%v).HTTP = %+v, want %+v", ep.ID, got.HTTP, wantHTTP)
		}

		stored, err := store.ByID(ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if stored.LastCheck != got {
			t.Errorf("ByID(%v).LastCheck = %+v, want %+v", ep.ID, stored.LastCheck, got)
		}
	})
}

// Test helpers

type fakeStore struct {
	mu        sync.RWMutex
	endpoints []Endpoint
}

func newTestService() (*Service, *fakeStore) {
	store := &fakeStore{}
	s := NewService(store)

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

	return s, store
}

func (store *fakeStore) Insert(endpoint Endpoint) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	for _, existing := range store.endpoints {
		if existing.URL == endpoint.URL {
			return ErrEndpointExists
		}
	}

	store.endpoints = append(store.endpoints, endpoint)

	return nil
}

func (store *fakeStore) List() ([]Endpoint, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	endpoints := make([]Endpoint, len(store.endpoints))
	copy(endpoints, store.endpoints)

	return endpoints, nil
}

func (store *fakeStore) ByID(id uuid.UUID) (Endpoint, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	for _, endpoint := range store.endpoints {
		if endpoint.ID == id {
			return endpoint, nil
		}
	}
	return Endpoint{}, ErrEndpointNotFound
}

func (store *fakeStore) RemoveByID(id uuid.UUID) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	for index, endpoint := range store.endpoints {
		if endpoint.ID == id {
			store.endpoints = append(store.endpoints[:index], store.endpoints[index+1:]...)
			return nil
		}
	}
	return ErrEndpointNotFound
}

func (store *fakeStore) UpdateLastCheck(id uuid.UUID, result CheckResult) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	for index, endpoint := range store.endpoints {
		if endpoint.ID == id {
			store.endpoints[index].LastCheck = result
			return nil
		}
	}

	return ErrEndpointNotFound
}
