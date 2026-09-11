package web

import (
	"context"
	"errors"
	"net/http"

	sessionpkg "github.com/sgrinan/signaldock/internal/session"
	"github.com/sgrinan/signaldock/internal/user"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'",
		)
		w.Header().Set("Referrer-Policy", "no-referrer")

		next.ServeHTTP(w, r)
	})
}

func (h *handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			http.Error(w, "failed to authenticate", http.StatusInternalServerError)
			return
		}

		session, err := h.sessions.Validate(r.Context(), cookie.Value)
		if err != nil {
			if errors.Is(err, sessionpkg.ErrSessionNotFound) ||
				errors.Is(err, sessionpkg.ErrSessionExpired) {

				clearSessionCookie(w, r)
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			h.logger.Error("failed to validate session", "error", err)

			http.Error(w, "failed to authenticate", http.StatusInternalServerError)
			return
		}

		account, err := h.users.ByID(r.Context(), session.UserID)
		if err != nil {
			if errors.Is(err, user.ErrUserNotFound) {
				clearSessionCookie(w, r)
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			h.logger.Error("failed to get authenticated user", "user_id", session.UserID, "error", err)

			http.Error(w, "failed to authenticate", http.StatusInternalServerError)
			return
		}

		if account.Disabled {
			if err := h.sessions.Delete(r.Context(), cookie.Value); err != nil {
				h.logger.Warn("failed to delete disabled user session", "user_id", account.ID, "error", err)
			}

			clearSessionCookie(w, r)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), currentUserKey, account)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, ok := currentUser(r)
		if !ok || account.Role != user.RoleAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
