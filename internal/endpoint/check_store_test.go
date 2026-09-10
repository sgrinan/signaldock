package endpoint

import (
	"net/http"
	"sync"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

func TestCheckStore_SetGet(t *testing.T) {
	store := NewCheckStore()

	id := uuid.NewV7()

	want := CheckResult{
		HTTP: probe.HTTPResult{
			StatusCode: http.StatusOK,
			Responded:  true,
		},
		TLS: probe.TLSResult{
			Enabled: true,
			Valid:   true,
		},
	}

	store.Set(id, want)

	got := store.Get(id)

	if got != want {
		t.Errorf("Get(%v) = %+v, want %+v", id, got, want)
	}
}

func TestCheckStore_GetMissing(t *testing.T) {
	store := NewCheckStore()

	id := uuid.NewV7()

	got := store.Get(id)

	if got != (CheckResult{}) {
		t.Errorf("Get(%v) = %+v, want zero CheckResult", id, got)
	}
}

func TestCheckStore_Delete(t *testing.T) {
	store := NewCheckStore()

	id := uuid.NewV7()

	store.Set(id, CheckResult{
		HTTP: probe.HTTPResult{
			StatusCode: http.StatusOK,
			Responded:  true,
		},
	})

	store.Delete(id)

	got := store.Get(id)

	if got != (CheckResult{}) {
		t.Errorf("Get(%v) = %+v, want zero CheckResult", id, got)
	}
}

func TestCheckStore_ConcurrentAccess(t *testing.T) {
	store := NewCheckStore()

	const goroutines = 20

	ids := make([]uuid.UUID, goroutines)

	for i := range goroutines {
		ids[i] = uuid.NewV7()
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func() {
			defer wg.Done()

			store.Set(ids[i], CheckResult{
				HTTP: probe.HTTPResult{
					StatusCode: http.StatusOK,
					Responded:  true,
				},
			})
		}()
	}

	wg.Wait()

	for _, id := range ids {
		got := store.Get(id)

		if !got.HTTP.Responded {
			t.Errorf("Get(%v).HTTP.Responded = false, want true", id)
		}
	}
}
