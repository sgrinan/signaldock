package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/session"
	"github.com/sgrinan/signaldock/internal/user"
)

func TestGrafanaProxyRequiresAuth(t *testing.T) {
	grafana := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	t.Cleanup(grafana.Close)

	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	handler, err := NewHandler(&fakeEndpointService{}, &fakeUserRepository{}, &fakeSessionService{}, grafana.URL, logger)
	if err != nil {
		t.Fatalf("NewHandler() error = %v, want nil", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/grafana/", nil)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusSeeOther; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}

	if got, want := recorder.Header().Get("Location"), "/login"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

func TestGrafanaProxyAuthenticated(t *testing.T) {
	userID := uuid.NewV7()

	reachedGrafana := false

	grafana := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reachedGrafana = true

			if got, want := r.URL.Path, "/grafana/"; got != want {
				t.Errorf("Grafana request path = %q, want %q", got, want)
			}

			w.WriteHeader(http.StatusOK)
		}),
	)
	t.Cleanup(grafana.Close)

	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	sessions := &fakeSessionService{
		validateFunc: func(_ context.Context, token string) (session.Session, error) {
			if got, want := token, "valid-token"; got != want {
				t.Errorf("Validate() token = %q, want %q", got, want)
			}

			return session.Session{
				UserID: userID,
			}, nil
		},
	}

	users := &fakeUserRepository{
		byIDFunc: func(_ context.Context, id uuid.UUID) (user.User, error) {
			if got, want := id, userID; got != want {
				t.Errorf("ByID() id = %v, want %v", got, want)
			}

			return user.User{
				ID:   userID,
				Role: user.RoleViewer,
			}, nil
		},
	}

	handler, err := NewHandler(&fakeEndpointService{}, users, sessions, grafana.URL, logger)
	if err != nil {
		t.Fatalf("NewHandler() error = %v, want nil", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/grafana/", nil)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "valid-token",
	})

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}

	if !reachedGrafana {
		t.Error("Grafana proxy did not forward authenticated request")
	}
}
