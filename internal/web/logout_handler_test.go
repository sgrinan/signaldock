package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandler_HandlePostLogout(t *testing.T) {
	h := newTestHandler(&fakeEndpointService{})

	deleted := false

	h.sessions = &fakeSessionService{
		deleteFunc: func(token string) error {
			if got, want := token, "session-token"; got != want {
				t.Errorf("Delete(%q), want %q", got, want)
			}

			deleted = true
			return nil
		},
	}

	form := url.Values{}
	form.Set("csrf_token", "csrf")

	req := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	req.AddCookie(&http.Cookie{
		Name:  csrfCookieName,
		Value: "csrf",
	})

	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	recorder := httptest.NewRecorder()

	h.handlePostLogout(recorder, req)

	if got, want := recorder.Code, http.StatusSeeOther; got != want {
		t.Errorf("handlePostLogout() status = %d, want %d", got, want)
	}

	if got, want := recorder.Header().Get("Location"), "/login"; got != want {
		t.Errorf("handlePostLogout() Location = %q, want %q", got, want)
	}

	if !deleted {
		t.Error("handlePostLogout() did not delete session")
	}

	var sessionCookie *http.Cookie

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("handlePostLogout() session cookie = nil, want cleared cookie")
	}

	if got, want := sessionCookie.MaxAge, -1; got != want {
		t.Errorf("session cookie MaxAge = %d, want %d", got, want)
	}
}

func TestHandler_HandlePostLogoutWithoutSession(t *testing.T) {
	h := newTestHandler(&fakeEndpointService{})

	form := url.Values{}
	form.Set("csrf_token", "csrf")

	req := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	req.AddCookie(&http.Cookie{
		Name:  csrfCookieName,
		Value: "csrf",
	})

	recorder := httptest.NewRecorder()

	h.handlePostLogout(recorder, req)

	if got, want := recorder.Code, http.StatusSeeOther; got != want {
		t.Errorf("handlePostLogout() status = %d, want %d", got, want)
	}

	if got, want := recorder.Header().Get("Location"), "/login"; got != want {
		t.Errorf("handlePostLogout() Location = %q, want %q", got, want)
	}
}
