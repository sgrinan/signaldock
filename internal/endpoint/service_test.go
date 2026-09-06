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

func newTestService() (*Service, *Store) {
	store := NewStore()
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
			t.Errorf("Add().LastCheck.HTTP.Responded = false, want true")
		}

		if !got.LastCheck.TLS.Valid {
			t.Errorf("Add().LastCheck.TLS.Valid = false, want true")
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

		got, err := s.Add("ftp://example.com")

		if !errors.Is(err, ErrUnsupportedScheme) {
			t.Errorf("Add(%q) error = %v, want %v", "ftp://example.com", err, ErrUnsupportedScheme)
		}

		if got != (Endpoint{}) {
			t.Errorf("Add() = %+v, want zero Endpoint", got)
		}

		if got := len(store.List()); got != 0 {
			t.Errorf("len(List()) = %d, want 0", got)
		}
	})

	t.Run("unsafe_host", func(t *testing.T) {
		s, store := newTestService()

		s.validateHost = func(string) ([]netip.Addr, error) {
			return nil, probe.ErrUnsafeHost
		}

		got, err := s.Add("https://example.com")

		if !errors.Is(err, ErrUnsafeHost) {
			t.Errorf("Add() error = %v, want %v", err, ErrUnsafeHost)
		}

		if got != (Endpoint{}) {
			t.Errorf("Add() = %+v, want zero Endpoint", got)
		}

		if got := len(store.List()); got != 0 {
			t.Errorf("len(List()) = %d, want 0", got)
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

		got, err := s.Add("https://example.com")

		var checkErr CheckError
		if !errors.As(err, &checkErr) {
			t.Fatalf("Add() error = %v, want CheckError", err)
		}

		if !errors.Is(err, httpErr) {
			t.Errorf("errors.Is(Add() error, httpErr) = false, want true")
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

		first, err := s.Add("https://example.com")
		if err != nil {
			t.Fatalf("first Add() returned unexpected error: %v", err)
		}

		_, err = s.Add("https://example.com")
		if !errors.Is(err, ErrEndpointExists) {
			t.Errorf("second Add() error = %v, want %v", err, ErrEndpointExists)
		}

		if got := len(store.List()); got != 1 {
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

	t.Run("concurrent_duplicate_runs_probes_once", func(t *testing.T) {
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

		var successCount int
		var duplicateCount int

		for err := range errs {
			switch {
			case err == nil:
				successCount++

			case errors.Is(err, ErrEndpointExists):
				duplicateCount++

			default:
				t.Errorf("Add() returned unexpected error: %v", err)
			}
		}

		if successCount != 1 {
			t.Errorf("successful Add() calls = %d, want 1", successCount)
		}

		if duplicateCount != goroutines-1 {
			t.Errorf("duplicate Add() calls = %d, want %d", duplicateCount, goroutines-1)
		}

		if got := len(store.List()); got != 1 {
			t.Errorf("len(List()) = %d, want 1", got)
		}

		if got := httpCalls.Load(); got != 1 {
			t.Errorf("HTTP probe calls = %d, want 1", got)
		}

		if got := tlsCalls.Load(); got != 1 {
			t.Errorf("TLS probe calls = %d, want 1", got)
		}
	})
}

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
