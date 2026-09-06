package web

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGenerateCSRFToken(t *testing.T) {
	token, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken() returned unexpected error: %v", err)
	}

	if got, want := len(token), 64; got != want {
		t.Errorf("len(generateCSRFToken()) = %d, want %d", got, want)
	}

	if _, err := hex.DecodeString(token); err != nil {
		t.Errorf("generateCSRFToken() = %q, want hexadecimal token", token)
	}
}

func TestGetCSRFToken(t *testing.T) {
	t.Run("creates_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		recorder := httptest.NewRecorder()

		token, err := getCSRFToken(recorder, req)
		if err != nil {
			t.Fatalf("getCSRFToken() returned unexpected error: %v", err)
		}

		if token == "" {
			t.Fatal("getCSRFToken() token is empty")
		}

		cookies := recorder.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("getCSRFToken() set %d cookies, want 1", len(cookies))
		}

		cookie := cookies[0]

		if cookie.Name != csrfCookieName {
			t.Errorf("CSRF cookie name = %q, want %q", cookie.Name, csrfCookieName)
		}

		if cookie.Value != token {
			t.Errorf("CSRF cookie value = %q, want %q", cookie.Value, token)
		}

		if cookie.Path != "/" {
			t.Errorf("CSRF cookie Path = %q, want %q", cookie.Path, "/")
		}

		if !cookie.HttpOnly {
			t.Error("CSRF cookie HttpOnly = false, want true")
		}

		if cookie.SameSite != http.SameSiteStrictMode {
			t.Errorf("CSRF cookie SameSite = %v, want %v", cookie.SameSite, http.SameSiteStrictMode)
		}

		if cookie.Secure {
			t.Error("CSRF cookie Secure = true for HTTP request, want false")
		}
	})

	t.Run("reuses_existing_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  csrfCookieName,
			Value: "existing-token",
		})

		recorder := httptest.NewRecorder()

		got, err := getCSRFToken(recorder, req)
		if err != nil {
			t.Fatalf("getCSRFToken() returned unexpected error: %v", err)
		}

		if got != "existing-token" {
			t.Errorf("getCSRFToken() = %q, want %q", got, "existing-token")
		}

		if cookies := recorder.Result().Cookies(); len(cookies) != 0 {
			t.Errorf("getCSRFToken() set %d cookies, want 0", len(cookies))
		}
	})

	t.Run("secure_over_tls", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
		recorder := httptest.NewRecorder()

		_, err := getCSRFToken(recorder, req)
		if err != nil {
			t.Fatalf("getCSRFToken() returned unexpected error: %v", err)
		}

		cookies := recorder.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("getCSRFToken() set %d cookies, want 1", len(cookies))
		}

		if !cookies[0].Secure {
			t.Error("CSRF cookie Secure = false for HTTPS request, want true")
		}
	})
}

func TestValidateCSRF(t *testing.T) {
	tests := []struct {
		name       string
		cookie     string
		formToken  string
		queryToken string
		want       bool
	}{
		{
			name:      "valid",
			cookie:    "secret-token",
			formToken: "secret-token",
			want:      true,
		},
		{
			name:      "missing_cookie",
			formToken: "secret-token",
			want:      false,
		},
		{
			name:   "missing_form_token",
			cookie: "secret-token",
			want:   false,
		},
		{
			name:      "mismatched_token",
			cookie:    "secret-token",
			formToken: "different-token",
			want:      false,
		},
		{
			name:       "query_token_not_accepted",
			cookie:     "secret-token",
			queryToken: "secret-token",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{}

			if tt.formToken != "" {
				form.Set(csrfFormField, tt.formToken)
			}

			target := "/"
			if tt.queryToken != "" {
				query := url.Values{}
				query.Set(csrfFormField, tt.queryToken)
				target += "?" + query.Encode()
			}

			req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{
					Name:  csrfCookieName,
					Value: tt.cookie,
				})
			}

			if got := validateCSRF(req); got != tt.want {
				t.Errorf("validateCSRF() = %t, want %t", got, tt.want)
			}
		})
	}
}
