package endpoint

import (
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

func TestCheckStore_SetGet(t *testing.T) {
	store := NewCheckStore()

	id := uuid.NewV7()

	want := CheckResult{
		HTTP: probe.HTTPResult{
			StatusCode: 200,
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
			StatusCode: 200,
			Responded:  true,
		},
	})

	store.Delete(id)

	got := store.Get(id)

	if got != (CheckResult{}) {
		t.Errorf("Get(%v) after Delete() = %+v, want zero CheckResult", id, got)
	}
}
