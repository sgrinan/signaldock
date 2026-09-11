package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/auth"
	"github.com/sgrinan/signaldock/internal/user"
)

func TestHandler_HandleGetLogin(t *testing.T) {
	h := newTestHandler(&fakeEndpointService{})

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	recorder := httptest.NewRecorder()

	h.handleGetLogin(recorder, req)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Errorf("handleGetLogin() status = %d, want %d", got, want)
	}

	if got := recorder.Body.String(); !strings.Contains(got, "csrf") {
		t.Errorf("handleGetLogin() body = %q, want CSRF token", got)
	}
}

func TestHandler_HandlePostLogin(t *testing.T) {
	password := "correct-password"
	passwordHash := auth.HashPassword(password)
	userID := uuid.NewV7()

	h := newTestHandler(&fakeEndpointService{})

	h.users = &fakeUserRepository{
		byUsernameFunc: func(username string) (user.User, error) {
			if got, want := username, "admin"; got != want {
				t.Errorf("ByUsername(%q), want %q", got, want)
			}

			return user.User{
				ID:           userID,
				Username:     "admin",
				PasswordHash: passwordHash,
				Role:         user.RoleAdmin,
			}, nil
		},
	}

	h.sessions = &fakeSessionService{
		createFunc: func(id uuid.UUID) (string, error) {
			if got, want := id, userID; got != want {
				t.Errorf("Create(%v), want %v", got, want)
			}

			return "session-token", nil
		},
	}

	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", password)
	form.Set("csrf_token", "csrf")

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  csrfCookieName,
		Value: "csrf",
	})

	recorder := httptest.NewRecorder()

	h.handlePostLogin(recorder, req)

	if got, want := recorder.Code, http.StatusSeeOther; got != want {
		t.Errorf("handlePostLogin() status = %d, want %d", got, want)
	}

	if got, want := recorder.Header().Get("Location"), "/"; got != want {
		t.Errorf("handlePostLogin() Location = %q, want %q", got, want)
	}

	var sessionCookie *http.Cookie

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("handlePostLogin() session cookie = nil, want cookie")
	}

	if got, want := sessionCookie.Value, "session-token"; got != want {
		t.Errorf("session cookie value = %q, want %q", got, want)
	}
}

func TestHandler_HandlePostLoginInvalidCredentials(t *testing.T) {
	passwordHash := auth.HashPassword("correct-password")

	tests := []struct {
		name  string
		users userRepository
	}{
		{
			name: "user_not_found",
			users: &fakeUserRepository{
				byUsernameFunc: func(string) (user.User, error) {
					return user.User{}, user.ErrUserNotFound
				},
			},
		},
		{
			name: "wrong_password",
			users: &fakeUserRepository{
				byUsernameFunc: func(string) (user.User, error) {
					return user.User{
						ID:           uuid.NewV7(),
						Username:     "admin",
						PasswordHash: passwordHash,
						Role:         user.RoleAdmin,
					}, nil
				},
			},
		},
		{
			name: "disabled_user",
			users: &fakeUserRepository{
				byUsernameFunc: func(string) (user.User, error) {
					return user.User{
						ID:           uuid.NewV7(),
						Username:     "admin",
						PasswordHash: auth.HashPassword("wrong-password"),
						Role:         user.RoleAdmin,
						Disabled:     true,
					}, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(&fakeEndpointService{})
			h.users = tt.users
			h.sessions = &fakeSessionService{}

			form := url.Values{}
			form.Set("username", "admin")
			form.Set("password", "wrong-password")
			form.Set("csrf_token", "csrf")

			req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{
				Name:  csrfCookieName,
				Value: "csrf",
			})

			recorder := httptest.NewRecorder()

			h.handlePostLogin(recorder, req)

			if got, want := recorder.Code, http.StatusUnauthorized; got != want {
				t.Errorf("handlePostLogin() status = %d, want %d", got, want)
			}

			if got := recorder.Body.String(); !strings.Contains(got, "Invalid username or password") {
				t.Errorf("handlePostLogin() body = %q, want invalid credentials error", got)
			}
		})
	}
}

func TestHandler_HandlePostLoginUserRepositoryError(t *testing.T) {
	h := newTestHandler(&fakeEndpointService{})

	h.users = &fakeUserRepository{
		byUsernameFunc: func(string) (user.User, error) {
			return user.User{}, errors.New("database error")
		},
	}

	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", "password")
	form.Set("csrf_token", "csrf")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  csrfCookieName,
		Value: "csrf",
	})

	recorder := httptest.NewRecorder()

	h.handlePostLogin(recorder, req)

	if got, want := recorder.Code, http.StatusInternalServerError; got != want {
		t.Errorf("handlePostLogin() status = %d, want %d", got, want)
	}
}

func TestHandler_HandlePostLoginSessionError(t *testing.T) {
	password := "correct-password"
	userID := uuid.NewV7()

	h := newTestHandler(&fakeEndpointService{})

	h.users = &fakeUserRepository{
		byUsernameFunc: func(string) (user.User, error) {
			return user.User{
				ID:           userID,
				Username:     "admin",
				PasswordHash: auth.HashPassword(password),
				Role:         user.RoleAdmin,
			}, nil
		},
	}

	h.sessions = &fakeSessionService{
		createFunc: func(uuid.UUID) (string, error) {
			return "", errors.New("database error")
		},
	}

	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", password)
	form.Set("csrf_token", "csrf")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  csrfCookieName,
		Value: "csrf",
	})

	recorder := httptest.NewRecorder()

	h.handlePostLogin(recorder, req)

	if got, want := recorder.Code, http.StatusInternalServerError; got != want {
		t.Errorf("handlePostLogin() status = %d, want %d", got, want)
	}
}
