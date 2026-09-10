package endpoint

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/database"
)

func TestRepository_Insert(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repository := newTestRepository(t)
		ep := testEndpoint("https://example.com/")

		if err := repository.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
		}

		got, err := repository.ByID(ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if got != ep {
			t.Errorf("ByID(%v) = %+v, want %+v", ep.ID, got, ep)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		repository := newTestRepository(t)

		first := testEndpoint("https://example.com/")
		second := testEndpoint("https://example.com/")

		if err := repository.Insert(first); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", first, err)
		}

		err := repository.Insert(second)
		if !errors.Is(err, ErrEndpointExists) {
			t.Errorf("Insert(%+v) error = %v, want %v", second, err, ErrEndpointExists)
		}

		endpoints, err := repository.List()
		if err != nil {
			t.Fatalf("List() returned unexpected error: %v", err)
		}

		if got, want := len(endpoints), 1; got != want {
			t.Errorf("len(List()) = %d, want %d", got, want)
		}
	})
}

func TestRepository_InsertConcurrentDuplicate(t *testing.T) {
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

			errs <- repository.Insert(testEndpoint("https://example.com/"))
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

	endpoints, err := repository.List()
	if err != nil {
		t.Fatalf("List() returned unexpected error: %v", err)
	}

	if got, want := len(endpoints), 1; got != want {
		t.Errorf("len(List()) = %d, want %d", got, want)
	}
}

func TestRepository_List(t *testing.T) {
	repository := newTestRepository(t)

	first := testEndpoint("https://example.com/")
	second := testEndpoint("https://example.org/")

	if err := repository.Insert(first); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", first, err)
	}

	if err := repository.Insert(second); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", second, err)
	}

	got, err := repository.List()
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

	repositoryd, err := repository.ByID(first.ID)
	if err != nil {
		t.Fatalf("ByID(%v) returned unexpected error: %v", first.ID, err)
	}

	if repositoryd.URL != first.URL {
		t.Errorf("ByID(%v).URL = %q, want %q", first.ID, repositoryd.URL, first.URL)
	}
}

func TestRepository_ByID(t *testing.T) {
	repository := newTestRepository(t)
	ep := testEndpoint("https://example.com/")

	if err := repository.Insert(ep); err != nil {
		t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
	}

	t.Run("found", func(t *testing.T) {
		got, err := repository.ByID(ep.ID)
		if err != nil {
			t.Fatalf("ByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if got != ep {
			t.Errorf("ByID(%v) = %+v, want %+v", ep.ID, got, ep)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		id := uuid.NewV7()

		if _, err := repository.ByID(id); !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("ByID(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
		}
	})
}

func TestRepository_RemoveByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repository := newTestRepository(t)
		ep := testEndpoint("https://example.com/")

		if err := repository.Insert(ep); err != nil {
			t.Fatalf("Insert(%+v) returned unexpected error: %v", ep, err)
		}

		if err := repository.RemoveByID(ep.ID); err != nil {
			t.Fatalf("RemoveByID(%v) returned unexpected error: %v", ep.ID, err)
		}

		if _, err := repository.ByID(ep.ID); !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("ByID(%v) error = %v, want %v", ep.ID, err, ErrEndpointNotFound)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		repository := newTestRepository(t)
		id := uuid.NewV7()

		if err := repository.RemoveByID(id); !errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("RemoveByID(%v) error = %v, want %v", id, err, ErrEndpointNotFound)
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

func newTestRepository(t *testing.T) *Repository {
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

	return NewRepository(pool)
}
