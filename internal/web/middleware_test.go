package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := securityHeaders(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	headers := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'",
		"Referrer-Policy":         "no-referrer",
	}

	for name, want := range headers {
		if got := recorder.Header().Get(name); got != want {
			t.Errorf("securityHeaders() %s = %q, want %q", name, got, want)
		}
	}

	if recorder.Code != http.StatusNoContent {
		t.Errorf("securityHeaders() status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
