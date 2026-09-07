package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// securityHeaders

func TestSecurityHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := securityHeaders(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	headers := []struct {
		name string
		want string
	}{
		{
			name: "X-Content-Type-Options",
			want: "nosniff",
		},
		{
			name: "Content-Security-Policy",
			want: "default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'",
		},
		{
			name: "Referrer-Policy",
			want: "no-referrer",
		},
	}

	for _, tt := range headers {
		if got := recorder.Header().Get(tt.name); got != tt.want {
			t.Errorf("securityHeaders() %s = %q, want %q", tt.name, got, tt.want)
		}
	}

	if got, want := recorder.Code, http.StatusNoContent; got != want {
		t.Errorf("securityHeaders() status = %d, want %d", got, want)
	}
}
