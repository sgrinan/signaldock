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
	"github.com/sgrinan/signaldock/internal/session"
	"github.com/sgrinan/signaldock/internal/user"
)

type fakeUserStore struct {
	insertFunc      func(user.User) error
	listFunc        func() ([]user.User, error)
	byUsernameFunc  func(string) (user.User, error)
	byIDFunc        func(uuid.UUID) (user.User, error)
	setDisabledFunc func(uuid.UUID, bool) error
	setRoleFunc     func(uuid.UUID, user.Role) error
	removeByIDFunc  func(uuid.UUID) error
}

func (f *fakeUserStore) Insert(account user.User) error {
	if f.insertFunc != nil {
		return f.insertFunc(account)
	}

	return nil
}

func (f *fakeUserStore) List() ([]user.User, error) {
	if f.listFunc != nil {
		return f.listFunc()
	}

	return nil, nil
}

func (f *fakeUserStore) ByUsername(username string) (user.User, error) {
	if f.byUsernameFunc != nil {
		return f.byUsernameFunc(username)
	}

	return user.User{}, user.ErrUserNotFound
}

func (f *fakeUserStore) ByID(id uuid.UUID) (user.User, error) {
	if f.byIDFunc != nil {
		return f.byIDFunc(id)
	}

	return user.User{}, user.ErrUserNotFound
}

func (f *fakeUserStore) SetDisabled(id uuid.UUID, disabled bool) error {
	if f.setDisabledFunc != nil {
		return f.setDisabledFunc(id, disabled)
	}

	return nil
}

func (f *fakeUserStore) SetRole(id uuid.UUID, role user.Role) error {
	if f.setRoleFunc != nil {
		return f.setRoleFunc(id, role)
	}

	return nil
}

func (f *fakeUserStore) RemoveByID(id uuid.UUID) error {
	if f.removeByIDFunc != nil {
		return f.removeByIDFunc(id)
	}

	return nil
}

type fakeSessionService struct {
	createFunc   func(uuid.UUID) (string, error)
	validateFunc func(string) (session.Session, error)
	deleteFunc   func(string) error
}

func (f *fakeSessionService) Create(userID uuid.UUID) (string, error) {
	if f.createFunc != nil {
		return f.createFunc(userID)
	}

	return "test-session-token", nil
}

func (f *fakeSessionService) Validate(token string) (session.Session, error) {
	if f.validateFunc != nil {
		return f.validateFunc(token)
	}

	return session.Session{}, nil
}

func (f *fakeSessionService) Delete(token string) error {
	if f.deleteFunc != nil {
		return f.deleteFunc(token)
	}

	return nil
}

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

	h.users = &fakeUserStore{
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
		users userStore
	}{
		{
			name: "user_not_found",
			users: &fakeUserStore{
				byUsernameFunc: func(string) (user.User, error) {
					return user.User{}, user.ErrUserNotFound
				},
			},
		},
		{
			name: "wrong_password",
			users: &fakeUserStore{
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
			users: &fakeUserStore{
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

func TestHandler_HandlePostLoginUserStoreError(t *testing.T) {
	h := newTestHandler(&fakeEndpointService{})

	h.users = &fakeUserStore{
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

	h.users = &fakeUserStore{
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
