package endpoint

import (
	"errors"
	"sync"
	"testing"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

func testEndpoint(rawURL string) Endpoint {
	return Endpoint{
		ID:  uuid.NewV7(),
		URL: rawURL,
	}
}

func TestStore_Insert(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := NewStore()
		ep := testEndpoint("https://example.com/")

		if err := store.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
		}

		got, err := store.ByID(ep.ID)
		if err != nil {
			t.Fatalf("GetByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if got != ep {
			t.Errorf("GetByID(%v) = %+v, want %+v", ep.ID, got, ep)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		store := NewStore()

		first := testEndpoint("https://example.com/")
		second := testEndpoint("https://example.com/")

		if err := store.Insert(first); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", first, err)
		}

		err := store.Insert(second)
		if !errors.Is(err, ErrEndpointExists) {
			t.Errorf("Insert(%+v) error = %v, want %v", second, err, ErrEndpointExists)
		}

		if got, want := len(store.List()), 1; got != want {
			t.Errorf("len(List()) = %d, want %d", got, want)
		}
	})
}

func TestStore_List(t *testing.T) {
	store := NewStore()

	first := testEndpoint("https://example.com/")
	second := testEndpoint("https://example.org/")

	if err := store.Insert(first); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", first, err)
	}

	if err := store.Insert(second); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", second, err)
	}

	got := store.List()

	if gotLen, wantLen := len(got), 2; gotLen != wantLen {
		t.Fatalf("len(List()) = %d, want %d", gotLen, wantLen)
	}

	if got[0] != first || got[1] != second {
		t.Errorf("List() = %+v, want [%+v %+v]", got, first, second)
	}

	got[0].URL = "https://modified.example/"

	stored, err := store.ByID(first.ID)
	if err != nil {
		t.Fatalf("GetByID(%v) returned unexpected error: %v", first.ID, err)
	}

	if stored.URL != first.URL {
		t.Errorf("GetByID(%v).URL = %q, want %q", first.ID, stored.URL, first.URL)
	}
}

func TestStore_GetByID(t *testing.T) {
	store := NewStore()
	ep := testEndpoint("https://example.com/")

	if err := store.Insert(ep); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
	}

	got, err := store.ByID(ep.ID)
	if err != nil {
		t.Fatalf("GetByID(%v) returned unexpected error: %v", ep.ID, err)
	}

	if got != ep {
		t.Errorf("GetByID(%v) = %+v, want %+v", ep.ID, got, ep)
	}

	id := uuid.NewV7()

	if _, err := store.ByID(id); !errors.Is(err, ErrEndpointNotFound) {
		t.Errorf("GetByID(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
	}
}

func TestStore_RemoveByID(t *testing.T) {
	store := NewStore()
	ep := testEndpoint("https://example.com/")

	if err := store.Insert(ep); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
	}

	if err := store.RemoveByID(ep.ID); err != nil {
		t.Fatalf("RemoveByID(%v) returned unexpected error: %v", ep.ID, err)
	}

	if _, err := store.ByID(ep.ID); !errors.Is(err, ErrEndpointNotFound) {
		t.Errorf("GetByID(%v) error = %v, want %v", ep.ID, err, ErrEndpointNotFound)
	}

	id := uuid.NewV7()

	if err := store.RemoveByID(id); !errors.Is(err, ErrEndpointNotFound) {
		t.Errorf("RemoveByID(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
	}
}

func TestStore_UpdateLastCheck(t *testing.T) {
	store := NewStore()
	ep := testEndpoint("https://example.com/")

	if err := store.Insert(ep); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
	}

	want := CheckResult{
		HTTP: probe.HTTPResult{
			StatusCode: 200,
			Responded:  true,
			Latency:    150 * time.Millisecond,
		},
		TLS: probe.TLSResult{
			Enabled: true,
			Valid:   true,
		},
	}

	if err := store.UpdateLastCheck(ep.ID, want); err != nil {
		t.Fatalf("UpdateLastCheck(%v, %+v) returned unexpected error: %v", ep.ID, want, err)
	}

	got, err := store.ByID(ep.ID)
	if err != nil {
		t.Fatalf("GetByID(%v) returned unexpected error: %v", ep.ID, err)
	}

	if got.LastCheck != want {
		t.Errorf("GetByID(%v).LastCheck = %+v, want %+v", ep.ID, got.LastCheck, want)
	}

	id := uuid.NewV7()

	if err := store.UpdateLastCheck(id, CheckResult{}); !errors.Is(err, ErrEndpointNotFound) {
		t.Errorf("UpdateLastCheck(%v, ...) error = %v, want %v", id, err, ErrEndpointNotFound)
	}
}

func TestStore_InsertConcurrentDuplicate(t *testing.T) {
	store := NewStore()

	const goroutines = 20

	start := make(chan struct{})
	errs := make(chan error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			<-start

			errs <- store.Insert(
				testEndpoint("https://example.com/"),
			)
		}()
	}

	close(start)

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
			t.Errorf("Insert() returned unexpected error: %v", err)
		}
	}

	if got, want := successes, 1; got != want {
		t.Errorf("successful Insert() calls = %d, want %d", got, want)
	}

	if got, want := duplicates, goroutines-1; got != want {
		t.Errorf("duplicate Insert() calls = %d, want %d", got, want)
	}

	if got, want := len(store.List()), 1; got != want {
		t.Errorf("len(List()) = %d, want %d", got, want)
	}
}
