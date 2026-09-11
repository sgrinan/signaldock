package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetSessionCookie(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantSecure bool
	}{
		{
			name:       "http",
			target:     "http://example.com",
			wantSecure: false,
		},
		{
			name:       "https",
			target:     "https://example.com",
			wantSecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			recorder := httptest.NewRecorder()

			setSessionCookie(recorder, req, "session-token")

			cookies := recorder.Result().Cookies()
			if got, want := len(cookies), 1; got != want {
				t.Fatalf("len(cookies) = %d, want %d", got, want)
			}

			cookie := cookies[0]

			if got, want := cookie.Name, sessionCookieName; got != want {
				t.Errorf("cookie Name = %q, want %q", got, want)
			}

			if got, want := cookie.Value, "session-token"; got != want {
				t.Errorf("cookie Value = %q, want %q", got, want)
			}

			if got, want := cookie.Secure, tt.wantSecure; got != want {
				t.Errorf("cookie Secure = %t, want %t", got, want)
			}

			if got, want := cookie.HttpOnly, true; got != want {
				t.Errorf("cookie HttpOnly = %t, want %t", got, want)
			}

			if got, want := cookie.SameSite, http.SameSiteLaxMode; got != want {
				t.Errorf("cookie SameSite = %v, want %v", got, want)
			}
		})
	}
}

func TestClearSessionCookie(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantSecure bool
	}{
		{
			name:       "http",
			target:     "http://example.com",
			wantSecure: false,
		},
		{
			name:       "https",
			target:     "https://example.com",
			wantSecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			recorder := httptest.NewRecorder()

			clearSessionCookie(recorder, req)

			cookies := recorder.Result().Cookies()
			if got, want := len(cookies), 1; got != want {
				t.Fatalf("len(cookies) = %d, want %d", got, want)
			}

			cookie := cookies[0]

			if got, want := cookie.MaxAge, -1; got != want {
				t.Errorf("cookie MaxAge = %d, want %d", got, want)
			}

			if got, want := cookie.Secure, tt.wantSecure; got != want {
				t.Errorf("cookie Secure = %t, want %t", got, want)
			}

			if got, want := cookie.HttpOnly, true; got != want {
				t.Errorf("cookie HttpOnly = %t, want %t", got, want)
			}
		})
	}
}
