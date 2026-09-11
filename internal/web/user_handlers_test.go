package web

import (
	"context"
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

func TestHandler_HandleGetUsers(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := newTestHandler(&fakeEndpointService{})

		current := user.User{
			ID:       uuid.NewV7(),
			Username: "admin",
			Role:     user.RoleAdmin,
		}

		h.users = &fakeUserRepository{
			listFunc: func(context.Context) ([]user.User, error) {
				return []user.User{
					current,
					{
						ID:       uuid.NewV7(),
						Username: "viewer",
						Role:     user.RoleViewer,
					},
				}, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req = withCurrentUser(req, current)

		recorder := httptest.NewRecorder()

		h.handleGetUsers(recorder, req)

		if got, want := recorder.Code, http.StatusOK; got != want {
			t.Errorf("handleGetUsers() status = %d, want %d", got, want)
		}

		if got, want := recorder.Body.String(), "|2|csrf"; got != want {
			t.Errorf("handleGetUsers() body = %q, want %q", got, want)
		}
	})

	t.Run("repository_error", func(t *testing.T) {
		h := newTestHandler(&fakeEndpointService{})

		repositoryErr := errors.New("repository failed")

		h.users = &fakeUserRepository{
			listFunc: func(context.Context) ([]user.User, error) {
				return nil, repositoryErr
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req = withCurrentUser(req, user.User{
			ID:   uuid.NewV7(),
			Role: user.RoleAdmin,
		})

		recorder := httptest.NewRecorder()

		h.handleGetUsers(recorder, req)

		if got, want := recorder.Code, http.StatusInternalServerError; got != want {
			t.Errorf("handleGetUsers() status = %d, want %d", got, want)
		}
	})
}

func TestHandler_HandlePostUser(t *testing.T) {
	h := newTestHandler(&fakeEndpointService{})

	var inserted user.User

	h.users = &fakeUserRepository{
		insertFunc: func(ctx context.Context, account user.User) error {
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

	req = withCurrentUser(req, user.User{
		ID:   uuid.NewV7(),
		Role: user.RoleAdmin,
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

			h.users = &fakeUserRepository{
				listFunc: func(context.Context) ([]user.User, error) {
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

			req = withCurrentUser(req, user.User{
				ID:       uuid.NewV7(),
				Username: "admin",
				Role:     user.RoleAdmin,
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

	h.users = &fakeUserRepository{
		insertFunc: func(context.Context, user.User) error {
			return user.ErrUserExists
		},
		listFunc: func(context.Context) ([]user.User, error) {
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

	req = withCurrentUser(req, user.User{
		ID:       uuid.NewV7(),
		Username: "admin",
		Role:     user.RoleAdmin,
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

	h.users = &fakeUserRepository{
		removeByIDFunc: func(context.Context, uuid.UUID) error {
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

	req = withCurrentUser(req, user.User{
		ID:   userID,
		Role: user.RoleAdmin,
	})

	recorder := httptest.NewRecorder()

	h.handleDeleteUser(recorder, req)

	if got, want := recorder.Code, http.StatusBadRequest; got != want {
		t.Errorf("handleDeleteUser() status = %d, want %d", got, want)
	}

	if called {
		t.Error("handleDeleteUser() called RemoveByID() for current user")
	}
}

func TestHandler_HandleDeleteUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := newTestHandler(&fakeEndpointService{})

		currentID := uuid.NewV7()
		targetID := uuid.NewV7()

		var removedID uuid.UUID

		h.users = &fakeUserRepository{
			removeByIDFunc: func(ctx context.Context, id uuid.UUID) error {
				removedID = id
				return nil
			},
		}

		req := newCSRFPostRequest(t, "/users/"+targetID.String()+"/delete", nil)
		req.SetPathValue("id", targetID.String())

		req = withCurrentUser(req, user.User{
			ID:   currentID,
			Role: user.RoleAdmin,
		})

		recorder := httptest.NewRecorder()

		h.handleDeleteUser(recorder, req)

		if got, want := recorder.Code, http.StatusSeeOther; got != want {
			t.Errorf("handleDeleteUser() status = %d, want %d", got, want)
		}

		if got, want := recorder.Header().Get("Location"), "/users"; got != want {
			t.Errorf("handleDeleteUser() Location = %q, want %q", got, want)
		}

		if got, want := removedID, targetID; got != want {
			t.Errorf("RemoveByID() id = %v, want %v", got, want)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		h := newTestHandler(&fakeEndpointService{})

		targetID := uuid.NewV7()

		h.users = &fakeUserRepository{
			removeByIDFunc: func(context.Context, uuid.UUID) error {
				return user.ErrUserNotFound
			},
		}

		req := newCSRFPostRequest(t, "/users/"+targetID.String()+"/delete", nil)
		req.SetPathValue("id", targetID.String())

		req = withCurrentUser(req, user.User{
			ID:   uuid.NewV7(),
			Role: user.RoleAdmin,
		})

		recorder := httptest.NewRecorder()

		h.handleDeleteUser(recorder, req)

		if got, want := recorder.Code, http.StatusNotFound; got != want {
			t.Errorf("handleDeleteUser() status = %d, want %d", got, want)
		}
	})
}

func TestHandler_HandleUpdateUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := newTestHandler(&fakeEndpointService{})

		currentID := uuid.NewV7()
		targetID := uuid.NewV7()

		var (
			gotRole     user.Role
			gotDisabled bool
		)

		h.users = &fakeUserRepository{
			setRoleFunc: func(ctx context.Context, id uuid.UUID, role user.Role) error {
				if got, want := id, targetID; got != want {
					t.Errorf("SetRole() id = %v, want %v", got, want)
				}

				gotRole = role
				return nil
			},

			setDisabledFunc: func(ctx context.Context, id uuid.UUID, disabled bool) error {
				if got, want := id, targetID; got != want {
					t.Errorf("SetDisabled() id = %v, want %v", got, want)
				}

				gotDisabled = disabled
				return nil
			},
		}

		form := url.Values{}
		form.Set("role", "admin")
		form.Set("disabled", "true")

		req := newCSRFPostRequest(t, "/users/"+targetID.String(), form)
		req.SetPathValue("id", targetID.String())

		req = withCurrentUser(req, user.User{
			ID:   currentID,
			Role: user.RoleAdmin,
		})

		recorder := httptest.NewRecorder()

		h.handleUpdateUser(recorder, req)

		if got, want := recorder.Code, http.StatusSeeOther; got != want {
			t.Errorf("handleUpdateUser() status = %d, want %d", got, want)
		}

		if got, want := recorder.Header().Get("Location"), "/users"; got != want {
			t.Errorf("handleUpdateUser() Location = %q, want %q", got, want)
		}

		if got, want := gotRole, user.RoleAdmin; got != want {
			t.Errorf("SetRole() role = %q, want %q", got, want)
		}

		if got, want := gotDisabled, true; got != want {
			t.Errorf("SetDisabled() disabled = %t, want %t", got, want)
		}
	})

	t.Run("self_update", func(t *testing.T) {
		h := newTestHandler(&fakeEndpointService{})

		currentID := uuid.NewV7()

		called := false

		h.users = &fakeUserRepository{
			setRoleFunc: func(context.Context, uuid.UUID, user.Role) error {
				called = true
				return nil
			},

			setDisabledFunc: func(context.Context, uuid.UUID, bool) error {
				called = true
				return nil
			},
		}

		form := url.Values{}
		form.Set("role", "viewer")
		form.Set("disabled", "true")

		req := newCSRFPostRequest(t, "/users/"+currentID.String(), form)
		req.SetPathValue("id", currentID.String())

		req = withCurrentUser(req, user.User{
			ID:   currentID,
			Role: user.RoleAdmin,
		})

		recorder := httptest.NewRecorder()

		h.handleUpdateUser(recorder, req)

		if got, want := recorder.Code, http.StatusBadRequest; got != want {
			t.Errorf("handleUpdateUser() status = %d, want %d", got, want)
		}

		if called {
			t.Error("handleUpdateUser() modified current user, want no repository call")
		}
	})

	t.Run("invalid_id", func(t *testing.T) {
		h := newTestHandler(&fakeEndpointService{})

		req := newCSRFPostRequest(t, "/users/invalid", url.Values{
			"role":     {"viewer"},
			"disabled": {"false"},
		},
		)
		req.SetPathValue("id", "invalid")

		req = withCurrentUser(req, user.User{
			ID:   uuid.NewV7(),
			Role: user.RoleAdmin,
		})

		recorder := httptest.NewRecorder()

		h.handleUpdateUser(recorder, req)

		if got, want := recorder.Code, http.StatusBadRequest; got != want {
			t.Errorf("handleUpdateUser() status = %d, want %d", got, want)
		}
	})

	t.Run("invalid_role", func(t *testing.T) {
		h := newTestHandler(&fakeEndpointService{})

		targetID := uuid.NewV7()

		req := newCSRFPostRequest(t, "/users/"+targetID.String(), url.Values{
			"role":     {"superadmin"},
			"disabled": {"false"},
		},
		)
		req.SetPathValue("id", targetID.String())

		req = withCurrentUser(req, user.User{
			ID:   uuid.NewV7(),
			Role: user.RoleAdmin,
		})

		recorder := httptest.NewRecorder()

		h.handleUpdateUser(recorder, req)

		if got, want := recorder.Code, http.StatusBadRequest; got != want {
			t.Errorf("handleUpdateUser() status = %d, want %d", got, want)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		h := newTestHandler(&fakeEndpointService{})

		targetID := uuid.NewV7()

		h.users = &fakeUserRepository{
			setRoleFunc: func(context.Context, uuid.UUID, user.Role) error {
				return user.ErrUserNotFound
			},
		}

		req := newCSRFPostRequest(t, "/users/"+targetID.String(), url.Values{
			"role":     {"viewer"},
			"disabled": {"false"},
		},
		)
		req.SetPathValue("id", targetID.String())

		req = withCurrentUser(req, user.User{
			ID:   uuid.NewV7(),
			Role: user.RoleAdmin,
		})

		recorder := httptest.NewRecorder()

		h.handleUpdateUser(recorder, req)

		if got, want := recorder.Code, http.StatusNotFound; got != want {
			t.Errorf("handleUpdateUser() status = %d, want %d", got, want)
		}
	})
}
