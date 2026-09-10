package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/auth"
	"github.com/sgrinan/signaldock/internal/user"
)

func TestHandler_HandlePostUser(t *testing.T) {
	h := newTestHandler(&fakeEndpointService{})

	var inserted user.User

	h.users = &fakeUserStore{
		insertFunc: func(account user.User) error {
			inserted = account
			return nil
		},
	}

	form := url.Values{}
	form.Set("username", "viewer")
	form.Set("password", "password123")
	form.Set("role", "viewer")
	form.Set("csrf_token", "csrf")

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  csrfCookieName,
		Value: "csrf",
	})

	recorder := httptest.NewRecorder()

	h.handlePostUser(recorder, req)

	if got, want := recorder.Code, http.StatusSeeOther; got != want {
		t.Errorf("handlePostUser() status = %d, want %d", got, want)
	}

	if got, want := recorder.Header().Get("Location"), "/users"; got != want {
		t.Errorf("handlePostUser() Location = %q, want %q", got, want)
	}

	if inserted.ID == (uuid.UUID{}) {
		t.Error("inserted ID = zero UUID, want generated UUID")
	}

	if got, want := inserted.Username, "viewer"; got != want {
		t.Errorf("inserted Username = %q, want %q", got, want)
	}

	if got, want := inserted.Role, user.RoleViewer; got != want {
		t.Errorf("inserted Role = %q, want %q", got, want)
	}

	ok, err := auth.VerifyPassword("password123", inserted.PasswordHash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}

	if !ok {
		t.Error("inserted PasswordHash does not match submitted password")
	}
}

func TestHandler_HandlePostUserInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		role     string
	}{
		{
			name:     "missing_username",
			password: "password123",
			role:     "viewer",
		},
		{
			name:     "missing_password",
			username: "viewer",
			role:     "viewer",
		},
		{
			name:     "invalid_role",
			username: "viewer",
			password: "password123",
			role:     "superadmin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(&fakeEndpointService{})

			h.users = &fakeUserStore{
				listFunc: func() ([]user.User, error) {
					return nil, nil
				},
			}

			form := url.Values{}
			form.Set("username", tt.username)
			form.Set("password", tt.password)
			form.Set("role", tt.role)
			form.Set("csrf_token", "csrf")

			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{
				Name:  csrfCookieName,
				Value: "csrf",
			})

			recorder := httptest.NewRecorder()

			h.handlePostUser(recorder, req)

			if got, want := recorder.Code, http.StatusBadRequest; got != want {
				t.Errorf("handlePostUser() status = %d, want %d", got, want)
			}
		})
	}
}

func TestHandler_HandlePostUserDuplicate(t *testing.T) {
	h := newTestHandler(&fakeEndpointService{})

	h.users = &fakeUserStore{
		insertFunc: func(user.User) error {
			return user.ErrUserExists
		},
		listFunc: func() ([]user.User, error) {
			return nil, nil
		},
	}

	form := url.Values{}
	form.Set("username", "viewer")
	form.Set("password", "password123")
	form.Set("role", "viewer")
	form.Set("csrf_token", "csrf")

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  csrfCookieName,
		Value: "csrf",
	})

	recorder := httptest.NewRecorder()

	h.handlePostUser(recorder, req)

	if got, want := recorder.Code, http.StatusBadRequest; got != want {
		t.Errorf("handlePostUser() status = %d, want %d", got, want)
	}

	if got := recorder.Body.String(); !strings.Contains(got, "Username already exists") {
		t.Errorf("handlePostUser() body = %q, want duplicate username error", got)
	}
}

func TestHandler_HandleDeleteUserSelf(t *testing.T) {
	userID := uuid.NewV7()

	h := newTestHandler(&fakeEndpointService{})

	called := false

	h.users = &fakeUserStore{
		removeByIDFunc: func(uuid.UUID) error {
			called = true
			return nil
		},
	}

	form := url.Values{}
	form.Set("csrf_token", "csrf")

	req := httptest.NewRequest(http.MethodPost, "/users/"+userID.String()+"/delete", strings.NewReader(form.Encode()))
	req.SetPathValue("id", userID.String())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  csrfCookieName,
		Value: "csrf",
	})

	ctx := context.WithValue(
		req.Context(),
		currentUserKey,
		user.User{
			ID:   userID,
			Role: user.RoleAdmin,
		},
	)

	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()

	h.handleDeleteUser(recorder, req)

	if got, want := recorder.Code, http.StatusBadRequest; got != want {
		t.Errorf("handleDeleteUser() status = %d, want %d", got, want)
	}

	if called {
		t.Error("handleDeleteUser() called RemoveByID() for current user")
	}
}
