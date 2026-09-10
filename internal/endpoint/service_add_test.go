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

func TestService_Add(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s, repository, checks := newTestService()

		got, err := s.Add("HTTPS://EXAMPLE.COM:443")
		if err != nil {
			t.Fatalf("Add() error = %v, want nil", err)
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

		stored, err := repository.ByID(got.ID)
		if err != nil {
			t.Fatalf("ByID(%v) error = %v, want nil", got.ID, err)
		}

		wantStored := Endpoint{
			ID:  got.ID,
			URL: got.URL,
		}

		if stored != wantStored {
			t.Errorf("ByID(%v) = %+v, want %+v", got.ID, stored, wantStored)
		}

		storedCheck := checks.Get(got.ID)

		if storedCheck != got.LastCheck {
			t.Errorf("Get(%v) = %+v, want %+v", got.ID, storedCheck, got.LastCheck)
		}
	})

	t.Run("invalid_url", func(t *testing.T) {
		s, repository, _ := newTestService()

		const rawURL = "ftp://example.com"

		got, err := s.Add(rawURL)

		if !errors.Is(err, ErrUnsupportedScheme) {
			t.Errorf("Add(%q) error = %v, want %v", rawURL, err, ErrUnsupportedScheme)
		}

		if got != (Endpoint{}) {
			t.Errorf("Add(%q) = %+v, want zero Endpoint", rawURL, got)
		}

		endpoints, err := repository.List()
		if err != nil {
			t.Fatalf("List() error = %v, want nil", err)
		}

		if got := len(endpoints); got != 0 {
			t.Errorf("len(List()) = %d, want 0", got)
		}
	})

	t.Run("unsafe_host", func(t *testing.T) {
		s, repository, _ := newTestService()

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

		endpoints, err := repository.List()
		if err != nil {
			t.Fatalf("List() error = %v, want nil", err)
		}

		if got := len(endpoints); got != 0 {
			t.Errorf("len(List()) = %d, want 0", got)
		}
	})

	t.Run("host_check_error_stores_failed_result", func(t *testing.T) {
		s, repository, checks := newTestService()

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

		stored, err := repository.ByID(got.ID)
		if err != nil {
			t.Fatalf("ByID(%v) error = %v, want nil", got.ID, err)
		}

		wantStored := Endpoint{
			ID:  got.ID,
			URL: got.URL,
		}

		if stored != wantStored {
			t.Errorf("ByID(%v) = %+v, want %+v", got.ID, stored, wantStored)
		}

		storedCheck := checks.Get(got.ID)

		if storedCheck != got.LastCheck {
			t.Errorf("Get(%v) = %+v, want %+v", got.ID, storedCheck, got.LastCheck)
		}
	})

	t.Run("check_error_stores_result", func(t *testing.T) {
		s, repository, checks := newTestService()

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

		stored, err := repository.ByID(got.ID)
		if err != nil {
			t.Fatalf("ByID(%v) error = %v, want nil", got.ID, err)
		}

		wantStored := Endpoint{
			ID:  got.ID,
			URL: got.URL,
		}

		if stored != wantStored {
			t.Errorf("ByID(%v) = %+v, want %+v", got.ID, stored, wantStored)
		}

		storedCheck := checks.Get(got.ID)

		if storedCheck != got.LastCheck {
			t.Errorf("Get(%v) = %+v, want %+v", got.ID, storedCheck, got.LastCheck)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		s, repository, checks := newTestService()

		const rawURL = "https://example.com"

		first, err := s.Add(rawURL)
		if err != nil {
			t.Fatalf("first Add(%q) error = %v, want nil", rawURL, err)
		}

		_, err = s.Add(rawURL)
		if !errors.Is(err, ErrEndpointExists) {
			t.Errorf("second Add(%q) error = %v, want %v", rawURL, err, ErrEndpointExists)
		}

		endpoints, err := repository.List()
		if err != nil {
			t.Fatalf("List() error = %v, want nil", err)
		}

		if got := len(endpoints); got != 1 {
			t.Errorf("len(List()) = %d, want 1", got)
		}

		stored, err := repository.ByID(first.ID)
		if err != nil {
			t.Fatalf("ByID(%v) error = %v, want nil", first.ID, err)
		}

		wantStored := Endpoint{
			ID:  first.ID,
			URL: first.URL,
		}

		if stored != wantStored {
			t.Errorf("ByID(%v) = %+v, want %+v", first.ID, stored, wantStored)
		}

		storedCheck := checks.Get(first.ID)

		if storedCheck != first.LastCheck {
			t.Errorf("Get(%v) = %+v, want %+v", first.ID, storedCheck, first.LastCheck)
		}
	})
}

func TestService_AddConcurrentDuplicate(t *testing.T) {
	s, repository, _ := newTestService()

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
			t.Errorf("Add(%v) error = %v, want nil or %v", "https://example.com/", err, ErrEndpointExists)
		}
	}

	if got, want := successes, 1; got != want {
		t.Errorf("successful Add() calls = %d, want %d", got, want)
	}

	if got, want := duplicates, goroutines-1; got != want {
		t.Errorf("duplicate Add() calls = %d, want %d", got, want)
	}

	endpoints, err := repository.List()
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
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
