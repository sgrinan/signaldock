package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	recorder := httptest.NewRecorder()

	handleHealth(recorder, req)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Errorf("handleHealth() status = %d, want %d", got, want)
	}
}
