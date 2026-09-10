package endpoint

import (
	"errors"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

func TestService_Refresh(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s, repository, checks := newTestService()

		ep := testEndpoint("https://example.com/")

		if err := repository.Insert(ep); err != nil {
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

		stored := checks.Get(ep.ID)

		if stored != want {
			t.Errorf("Get(%v) = %+v, want %+v", ep.ID, stored, want)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		s, _, _ := newTestService()
		id := uuid.NewV7()

		_, err := s.Refresh(id)

		if !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("Refresh(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
		}
	})

	t.Run("host_check_error_updates_result", func(t *testing.T) {
		s, repository, checks := newTestService()

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

		if err := repository.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) error = %v, want nil", ep, err)
		}

		// Repository only persists endpoint configuration.
		// The previous runtime check belongs in CheckStore.
		checks.Set(ep.ID, ep.LastCheck)

		s.validateHost = func(string) ([]netip.Addr, error) {
			return nil, probe.ErrNoResolvedAddresses
		}

		got, err := s.Refresh(ep.ID)

		var checkErr CheckError
		if !errors.As(err, &checkErr) {
			t.Fatalf("Refresh(%v) error = %v, want CheckError", ep.ID, err)
		}

		if !errors.Is(err, probe.ErrNoResolvedAddresses) {
			t.Errorf(
				"Refresh(%v) error = %v, want %v",
				ep.ID,
				err,
				probe.ErrNoResolvedAddresses,
			)
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

		stored := checks.Get(ep.ID)

		if stored != got {
			t.Errorf("Get(%v) = %+v, want %+v", ep.ID, stored, got)
		}
	})

	t.Run("check_error_updates_result", func(t *testing.T) {
		s, repository, checks := newTestService()

		ep := testEndpoint("https://example.com/")

		if err := repository.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) error = %v, want nil", ep, err)
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
			t.Errorf(
				"Refresh(%v).HTTP = %+v, want %+v",
				ep.ID,
				got.HTTP,
				wantHTTP,
			)
		}

		stored := checks.Get(ep.ID)

		if stored != got {
			t.Errorf("Get(%v) = %+v, want %+v", ep.ID, stored, got)
		}
	})
}
