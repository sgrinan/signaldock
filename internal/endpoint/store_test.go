package endpoint

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/database"
	"github.com/sgrinan/signaldock/internal/probe"
)

// Store.Insert

func TestStore_Insert(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := newTestStore(t)
		ep := testEndpoint("https://example.com/")

		if err := store.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
		}

		got, err := store.ByID(ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if got != ep {
			t.Errorf("ByID(%v) = %+v, want %+v", ep.ID, got, ep)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		store := newTestStore(t)

		first := testEndpoint("https://example.com/")
		second := testEndpoint("https://example.com/")

		if err := store.Insert(first); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", first, err)
		}

		err := store.Insert(second)
		if !errors.Is(err, ErrEndpointExists) {
			t.Errorf("Insert(%+v) error = %v, want %v", second, err, ErrEndpointExists)
		}

		endpoints, err := store.List()
		if err != nil {
			t.Fatalf("List() returned unexpected error: %v", err)
		}

		if got, want := len(endpoints), 1; got != want {
			t.Errorf("len(List()) = %d, want %d", got, want)
		}
	})
}

func TestStore_InsertConcurrentDuplicate(t *testing.T) {
	store := newTestStore(t)

	const goroutines = 20

	start := make(chan struct{})
	errs := make(chan error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			<-start

			errs <- store.Insert(testEndpoint("https://example.com/"))
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

	endpoints, err := store.List()
	if err != nil {
		t.Fatalf("List() returned unexpected error: %v", err)
	}

	if got, want := len(endpoints), 1; got != want {
		t.Errorf("len(List()) = %d, want %d", got, want)
	}
}

// Store.List

func TestStore_List(t *testing.T) {
	store := newTestStore(t)

	first := testEndpoint("https://example.com/")
	second := testEndpoint("https://example.org/")

	if err := store.Insert(first); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", first, err)
	}

	if err := store.Insert(second); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", second, err)
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("List() returned unexpected error: %v", err)
	}

	if gotLen, wantLen := len(got), 2; gotLen != wantLen {
		t.Fatalf("len(List()) = %d, want %d", gotLen, wantLen)
	}

	if got[0] != first || got[1] != second {
		t.Errorf("List() = %+v, want [%+v %+v]", got, first, second)
	}

	got[0].URL = "https://modified.example/"

	stored, err := store.ByID(first.ID)
	if err != nil {
		t.Fatalf("ByID(%v) returned unexpected error: %v", first.ID, err)
	}

	if stored.URL != first.URL {
		t.Errorf("ByID(%v).URL = %q, want %q", first.ID, stored.URL, first.URL)
	}
}

// Store.ByID

func TestStore_ByID(t *testing.T) {
	store := newTestStore(t)
	ep := testEndpoint("https://example.com/")

	if err := store.Insert(ep); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
	}

	t.Run("found", func(t *testing.T) {
		got, err := store.ByID(ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if got != ep {
			t.Errorf("ByID(%v) = %+v, want %+v", ep.ID, got, ep)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		id := uuid.NewV7()

		if _, err := store.ByID(id); !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("ByID(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
		}
	})
}

// Store.RemoveByID

func TestStore_RemoveByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := newTestStore(t)
		ep := testEndpoint("https://example.com/")

		if err := store.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
		}

		if err := store.RemoveByID(ep.ID); err != nil {
			t.Fatalf("RemoveByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if _, err := store.ByID(ep.ID); !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("ByID(%v) error = %v, want %v", ep.ID, err, ErrEndpointNotFound)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		store := newTestStore(t)
		id := uuid.NewV7()

		if err := store.RemoveByID(id); !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("RemoveByID(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
		}
	})
}

// Store.UpdateLastCheck

func TestStore_UpdateLastCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := newTestStore(t)
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
			t.Fatalf("ByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if got.LastCheck != want {
			t.Errorf("ByID(%v).LastCheck = %+v, want %+v", ep.ID, got.LastCheck, want)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		store := newTestStore(t)
		id := uuid.NewV7()

		if err := store.UpdateLastCheck(id, CheckResult{}); !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("UpdateLastCheck(%v, ...) error = %v, want %v", id, err, ErrEndpointNotFound)
		}
	})
}

// Test helpers

func testEndpoint(rawURL string) Endpoint {
	return Endpoint{
		ID:  uuid.NewV7(),
		URL: rawURL,
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	databaseURL := os.Getenv("SIGNALDOCK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("SIGNALDOCK_TEST_DATABASE_URL is required")
	}

	ctx := context.Background()

	pool, err := database.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresPool() error = %v", err)
	}

	t.Cleanup(pool.Close)

	if err := database.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	if _, err := pool.Exec(ctx, `TRUNCATE TABLE endpoints`); err != nil {
		t.Fatalf("TRUNCATE endpoints error = %v", err)
	}

	return NewStore(pool)
}
