package endpoint

import (
	"errors"
	"net/http"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

func TestService_List(t *testing.T) {
	s, repository, checks := newTestService()

	first := testEndpoint("https://example.com/")
	second := testEndpoint("https://example.org/")

	if err := repository.Insert(first); err != nil {
		t.Fatalf("Insert(%+v) error = %v, want nil", first, err)
	}

	if err := repository.Insert(second); err != nil {
		t.Fatalf("Insert(%+v) error = %v, want nil", second, err)
	}

	firstCheck := CheckResult{
		HTTP: probe.HTTPResult{
			StatusCode: http.StatusOK,
			Responded:  true,
		},
		TLS: probe.TLSResult{
			Enabled: true,
			Valid:   true,
		},
	}

	secondCheck := CheckResult{
		HTTP: probe.HTTPResult{
			StatusCode: http.StatusServiceUnavailable,
			Responded:  true,
		},
		TLS: probe.TLSResult{
			Enabled: true,
			Valid:   true,
		},
	}

	checks.Set(first.ID, firstCheck)
	checks.Set(second.ID, secondCheck)

	first.LastCheck = firstCheck
	second.LastCheck = secondCheck

	want := []Endpoint{
		first,
		second,
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}

	if gotLen, wantLen := len(got), len(want); gotLen != wantLen {
		t.Fatalf("len(List()) = %d, want %d", gotLen, wantLen)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestService_ByID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		s, repository, checks := newTestService()

		ep := testEndpoint("https://example.com/")

		if err := repository.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) error = %v, want nil", ep, err)
		}

		check := CheckResult{
			HTTP: probe.HTTPResult{
				StatusCode: http.StatusOK,
				Responded:  true,
			},
			TLS: probe.TLSResult{
				Enabled: true,
				Valid:   true,
			},
		}

		checks.Set(ep.ID, check)

		ep.LastCheck = check

		got, err := s.ByID(ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) error = %v, want nil", ep.ID, err)
		}

		if got != ep {
			t.Errorf("ByID(%v) = %+v, want %+v", ep.ID, got, ep)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		s, _, _ := newTestService()

		id := uuid.NewV7()

		got, err := s.ByID(id)

		if !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("ByID(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
		}

		if got != (Endpoint{}) {
			t.Errorf("ByID(%v) = %+v, want zero Endpoint", id, got)
		}
	})
}

func TestService_RemoveByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s, repository, checks := newTestService()

		ep := testEndpoint("https://example.com/")

		if err := repository.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) error = %v, want nil", ep, err)
		}

		check := CheckResult{
			HTTP: probe.HTTPResult{
				StatusCode: http.StatusOK,
				Responded:  true,
			},
		}

		checks.Set(ep.ID, check)

		if err := s.RemoveByID(ep.ID); err != nil {
			t.Fatalf("RemoveByID(%v) error = %v, want nil", ep.ID, err)
		}

		_, err := repository.ByID(ep.ID)
		if !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("repository.ByID(%v) error = %v, want %v", ep.ID, err, ErrEndpointNotFound)
		}

		if got := checks.Get(ep.ID); got != (CheckResult{}) {
			t.Errorf("checks.Get(%v) = %+v, want zero CheckResult", ep.ID, got)
		}
	})

	t.Run("not_found_preserves_check", func(t *testing.T) {
		s, _, checks := newTestService()

		id := uuid.NewV7()

		want := CheckResult{
			HTTP: probe.HTTPResult{
				StatusCode: http.StatusOK,
				Responded:  true,
			},
		}

		checks.Set(id, want)

		err := s.RemoveByID(id)

		if !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("RemoveByID(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
		}

		got := checks.Get(id)

		if got != want {
			t.Errorf("checks.Get(%v) = %+v, want %+v", id, got, want)
		}
	})
}
