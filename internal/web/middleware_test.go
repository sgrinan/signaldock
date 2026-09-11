package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/session"
	"github.com/sgrinan/signaldock/internal/user"
)

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

func TestHandler_RequireAuth(t *testing.T) {
	userID := uuid.NewV7()

	tests := []struct {
		name       string
		cookie     *http.Cookie
		sessions   sessionService
		users      userRepository
		wantStatus int
		wantNext   bool
		wantUser   bool
	}{
		{
			name:       "missing_cookie",
			wantStatus: http.StatusSeeOther,
		},
		{
			name: "valid_session",
			cookie: &http.Cookie{
				Name:  sessionCookieName,
				Value: "session-token",
			},
			sessions: &fakeSessionService{
				validateFunc: func(ctx context.Context, token string) (session.Session, error) {
					if got, want := token, "session-token"; got != want {
						t.Errorf("Validate(%q), want %q", got, want)
					}

					return session.Session{
						UserID: userID,
					}, nil
				},
			},
			users: &fakeUserRepository{
				byIDFunc: func(ctx context.Context, id uuid.UUID) (user.User, error) {
					if got, want := id, userID; got != want {
						t.Errorf("ByID(%v), want %v", got, want)
					}

					return user.User{
						ID:       userID,
						Username: "admin",
						Role:     user.RoleAdmin,
					}, nil
				},
			},
			wantStatus: http.StatusOK,
			wantNext:   true,
			wantUser:   true,
		},
		{
			name: "session_not_found",
			cookie: &http.Cookie{
				Name:  sessionCookieName,
				Value: "session-token",
			},
			sessions: &fakeSessionService{
				validateFunc: func(context.Context, string) (session.Session, error) {
					return session.Session{}, session.ErrSessionNotFound
				},
			},
			wantStatus: http.StatusSeeOther,
		},
		{
			name: "session_expired",
			cookie: &http.Cookie{
				Name:  sessionCookieName,
				Value: "session-token",
			},
			sessions: &fakeSessionService{
				validateFunc: func(context.Context, string) (session.Session, error) {
					return session.Session{}, session.ErrSessionExpired
				},
			},
			wantStatus: http.StatusSeeOther,
		},
		{
			name: "user_not_found",
			cookie: &http.Cookie{
				Name:  sessionCookieName,
				Value: "session-token",
			},
			sessions: &fakeSessionService{
				validateFunc: func(context.Context, string) (session.Session, error) {
					return session.Session{
						UserID: userID,
					}, nil
				},
			},
			users: &fakeUserRepository{
				byIDFunc: func(context.Context, uuid.UUID) (user.User, error) {
					return user.User{}, user.ErrUserNotFound
				},
			},
			wantStatus: http.StatusSeeOther,
		},
		{
			name: "disabled_user",
			cookie: &http.Cookie{
				Name:  sessionCookieName,
				Value: "session-token",
			},
			sessions: &fakeSessionService{
				validateFunc: func(context.Context, string) (session.Session, error) {
					return session.Session{
						UserID: userID,
					}, nil
				},
			},
			users: &fakeUserRepository{
				byIDFunc: func(context.Context, uuid.UUID) (user.User, error) {
					return user.User{
						ID:       userID,
						Username: "viewer",
						Role:     user.RoleViewer,
						Disabled: true,
					}, nil
				},
			},
			wantStatus: http.StatusSeeOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(&fakeEndpointService{})

			if tt.sessions != nil {
				h.sessions = tt.sessions
			}

			if tt.users != nil {
				h.users = tt.users
			}

			nextCalled := false
			userFound := false

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true

				if _, ok := currentUser(r); ok {
					userFound = true
				}

				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)

			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}

			recorder := httptest.NewRecorder()

			h.requireAuth(next).ServeHTTP(recorder, req)

			if got := recorder.Code; got != tt.wantStatus {
				t.Errorf("requireAuth() status = %d, want %d", got, tt.wantStatus)
			}

			if got := nextCalled; got != tt.wantNext {
				t.Errorf("requireAuth() next called = %v, want %v", got, tt.wantNext)
			}

			if got := userFound; got != tt.wantUser {
				t.Errorf("requireAuth() current user found = %v, want %v", got, tt.wantUser)
			}

			if tt.wantStatus == http.StatusSeeOther {
				if got, want := recorder.Header().Get("Location"), "/login"; got != want {
					t.Errorf("requireAuth() Location = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestHandler_RequireAuthSessionError(t *testing.T) {
	wantErr := errors.New("database error")

	h := newTestHandler(&fakeEndpointService{})
	h.sessions = &fakeSessionService{
		validateFunc: func(context.Context, string) (session.Session, error) {
			return session.Session{}, wantErr
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	recorder := httptest.NewRecorder()

	h.requireAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler called, want authentication failure")
	})).ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusInternalServerError; got != want {
		t.Errorf("requireAuth() status = %d, want %d", got, want)
	}
}

func TestHandler_RequireAuthUserRepositoryError(t *testing.T) {
	userID := uuid.NewV7()

	h := newTestHandler(&fakeEndpointService{})

	h.sessions = &fakeSessionService{
		validateFunc: func(context.Context, string) (session.Session, error) {
			return session.Session{
				UserID: userID,
			}, nil
		},
	}

	h.users = &fakeUserRepository{
		byIDFunc: func(context.Context, uuid.UUID) (user.User, error) {
			return user.User{}, errors.New("database error")
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	recorder := httptest.NewRecorder()

	h.requireAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler called, want authentication failure")
	})).ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusInternalServerError; got != want {
		t.Errorf("requireAuth() status = %d, want %d", got, want)
	}
}

func TestHandler_RequireAdmin(t *testing.T) {
	tests := []struct {
		name       string
		account    *user.User
		wantStatus int
		wantNext   bool
	}{
		{
			name: "admin",
			account: &user.User{
				ID:   uuid.NewV7(),
				Role: user.RoleAdmin,
			},
			wantStatus: http.StatusOK,
			wantNext:   true,
		},
		{
			name: "viewer",
			account: &user.User{
				ID:   uuid.NewV7(),
				Role: user.RoleViewer,
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing_user",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(&fakeEndpointService{})

			nextCalled := false

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodPost, "/endpoints", nil)

			if tt.account != nil {
				ctx := context.WithValue(req.Context(), currentUserKey, *tt.account)
				req = req.WithContext(ctx)
			}

			recorder := httptest.NewRecorder()

			h.requireAdmin(next).ServeHTTP(recorder, req)

			if got := recorder.Code; got != tt.wantStatus {
				t.Errorf("requireAdmin() status = %d, want %d", got, tt.wantStatus)
			}

			if got := nextCalled; got != tt.wantNext {
				t.Errorf("requireAdmin() next called = %v, want %v", got, tt.wantNext)
			}
		})
	}
}
