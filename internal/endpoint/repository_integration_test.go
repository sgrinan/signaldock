package endpoint

import (
	"errors"
	"net/http"
	"os"
	"sync"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/database"
)

func TestRepository_Insert(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := t.Context()

		repository := newTestRepository(t)
		ep := testEndpoint("https://example.com/")

		if err := repository.Insert(ctx, ep); err != nil {
			t.Fatalf("Insert(%+v) error = %v, want nil", ep, err)
		}

		got, err := repository.ByID(ctx, ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) error = %v, want nil", ep.ID, err)
		}

		if got != ep {
			t.Errorf("ByID(%v) = %+v, want %+v", ep.ID, got, ep)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		ctx := t.Context()

		repository := newTestRepository(t)

		first := testEndpoint("https://example.com/")
		second := testEndpoint("https://example.com/")

		if err := repository.Insert(ctx, first); err != nil {
			t.Fatalf("Insert(%+v) error = %v, want nil", first, err)
		}

		err := repository.Insert(ctx, second)
		if !errors.Is(err, ErrEndpointExists) {
			t.Errorf("Insert(%+v) error = %v, want %v", second, err, ErrEndpointExists)
		}

		endpoints, err := repository.List(ctx)
		if err != nil {
			t.Fatalf("List() error = %v, want nil", err)
		}

		if got, want := len(endpoints), 1; got != want {
			t.Errorf("len(List()) = %d, want %d", got, want)
		}
	})
	t.Run("does_not_persist_last_check", func(t *testing.T) {
		ctx := t.Context()

		repository := newTestRepository(t)

		ep := testEndpoint("https://example.com/")

		ep.LastCheck.HTTP.StatusCode = http.StatusOK
		ep.LastCheck.HTTP.Responded = true
		ep.LastCheck.TLS.Enabled = true
		ep.LastCheck.TLS.Valid = true

		if err := repository.Insert(ctx, ep); err != nil {
			t.Fatalf("Insert(%+v) error = %v, want nil", ep, err)
		}

		got, err := repository.ByID(ctx, ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) error = %v, want nil", ep.ID, err)
		}

		if got.LastCheck != (CheckResult{}) {
			t.Errorf("ByID(%v).LastCheck = %+v, want zero CheckResult", ep.ID, got.LastCheck)
		}
	})
}

func TestRepository_InsertConcurrentDuplicate(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)

	const goroutines = 20

	start := make(chan struct{})
	errs := make(chan error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			<-start

			errs <- repository.Insert(ctx, testEndpoint("https://example.com/"))
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
			t.Errorf("Insert(%q) error = %v, want nil or %v", "https://example.com/", err, ErrEndpointExists)
		}
	}

	if got, want := successes, 1; got != want {
		t.Errorf("successful Insert() calls = %d, want %d", got, want)
	}

	if got, want := duplicates, goroutines-1; got != want {
		t.Errorf("duplicate Insert() calls = %d, want %d", got, want)
	}

	endpoints, err := repository.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}

	if got, want := len(endpoints), 1; got != want {
		t.Errorf("len(List()) = %d, want %d", got, want)
	}
}

func TestRepository_List(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)

	first := testEndpoint("https://example.com/")
	second := testEndpoint("https://example.org/")

	if err := repository.Insert(ctx, first); err != nil {
		t.Fatalf("Insert(%+v) error = %v, want nil", first, err)
	}

	if err := repository.Insert(ctx, second); err != nil {
		t.Fatalf("Insert(%+v) error = %v, want nil", second, err)
	}

	got, err := repository.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}

	if gotLen, wantLen := len(got), 2; gotLen != wantLen {
		t.Fatalf("len(List()) = %d, want %d", gotLen, wantLen)
	}

	if got[0] != first || got[1] != second {
		t.Errorf("List() = %+v, want [%+v %+v]", got, first, second)
	}

	got[0].URL = "https://modified.example/"

	stored, err := repository.ByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("ByID(%v) error = %v, want nil", first.ID, err)
	}

	if stored.URL != first.URL {
		t.Errorf("ByID(%v).URL = %q, want %q", first.ID, stored.URL, first.URL)
	}
}

func TestRepository_ByID(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)
	ep := testEndpoint("https://example.com/")

	if err := repository.Insert(ctx, ep); err != nil {
		t.Fatalf("Insert(%+v) error = %v, want nil", ep, err)
	}

	t.Run("found", func(t *testing.T) {
		ctx := t.Context()

		got, err := repository.ByID(ctx, ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) error = %v, want nil", ep.ID, err)
		}

		if got != ep {
			t.Errorf("ByID(%v) = %+v, want %+v", ep.ID, got, ep)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		ctx := t.Context()

		id := uuid.NewV7()

		if _, err := repository.ByID(ctx, id); !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("ByID(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
		}
	})
}

func TestRepository_RemoveByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := t.Context()

		repository := newTestRepository(t)
		ep := testEndpoint("https://example.com/")

		if err := repository.Insert(ctx, ep); err != nil {
			t.Fatalf("Insert(%+v) error = %v, want nil", ep, err)
		}

		if err := repository.RemoveByID(ctx, ep.ID); err != nil {
			t.Fatalf("RemoveByID(%v) error = %v, want nil", ep.ID, err)
		}

		if _, err := repository.ByID(ctx, ep.ID); !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("ByID(%v) error = %v, want %v", ep.ID, err, ErrEndpointNotFound)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		ctx := t.Context()

		repository := newTestRepository(t)
		id := uuid.NewV7()

		if err := repository.RemoveByID(ctx, id); !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("RemoveByID(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
		}
	})
}

// Test helpers

func newTestRepository(t *testing.T) *Repository {
	t.Helper()

	databaseURL := os.Getenv("SIGNALDOCK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("SIGNALDOCK_TEST_DATABASE_URL is required")
	}

	ctx := t.Context()

	pool, err := database.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresPool() error = %v, want nil", err)
	}

	t.Cleanup(pool.Close)

	if err := database.EnsureSchema(ctx, pool); err != nil {
		t.Fatalf("EnsureSchema() error = %v, want nil", err)
	}

	if _, err := pool.Exec(ctx, `TRUNCATE TABLE endpoints`); err != nil {
		t.Fatalf("TRUNCATE endpoints error = %v, want nil", err)
	}

	return NewRepository(pool)
}
