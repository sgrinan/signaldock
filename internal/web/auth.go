package web

import (
	"context"
	"net/http"

	"uuid"

	"github.com/sgrinan/signaldock/internal/session"
	"github.com/sgrinan/signaldock/internal/user"
)

const sessionCookieName = "signaldock_session"

type contextKey string

const currentUserKey contextKey = "current-user"

type userRepository interface {
	Insert(context.Context, user.User) error
	List(context.Context) ([]user.User, error)
	ByUsername(context.Context, string) (user.User, error)
	ByID(context.Context, uuid.UUID) (user.User, error)
	SetDisabled(context.Context, uuid.UUID, bool) error
	SetRole(context.Context, uuid.UUID, user.Role) error
	RemoveByID(context.Context, uuid.UUID) error
}

type sessionService interface {
	Create(context.Context, uuid.UUID) (string, error)
	Validate(context.Context, string) (session.Session, error)
	Delete(context.Context, string) error
}

type loginPageData struct {
	Error     string
	CSRFToken string
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   -1,
	})
}

func currentUser(r *http.Request) (user.User, bool) {
	account, ok := r.Context().Value(currentUserKey).(user.User)

	return account, ok
}
