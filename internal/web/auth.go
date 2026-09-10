package web

import (
	"net/http"

	"uuid"

	"github.com/sgrinan/signaldock/internal/session"
	"github.com/sgrinan/signaldock/internal/user"
)

const sessionCookieName = "signaldock_session"

type contextKey string

const currentUserKey contextKey = "current-user"

type userStore interface {
	ByUsername(string) (user.User, error)
	ByID(uuid.UUID) (user.User, error)
}

type sessionService interface {
	Create(uuid.UUID) (string, error)
	Validate(string) (session.Session, error)
	Delete(string) error
}

type loginPageData struct {
	Error     string
	CSRFToken string
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func currentUser(r *http.Request) (user.User, bool) {
	account, ok := r.Context().Value(currentUserKey).(user.User)

	return account, ok
}
