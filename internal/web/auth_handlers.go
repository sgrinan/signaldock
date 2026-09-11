package web

import (
	"errors"
	"net/http"

	"github.com/sgrinan/signaldock/internal/auth"
	"github.com/sgrinan/signaldock/internal/user"
)

func (h *handler) handleGetLogin(w http.ResponseWriter, r *http.Request) {
	csrfToken, err := getCSRFToken(w, r)
	if err != nil {
		h.logger.Error("failed to get CSRF token", "error", err)

		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}

	data := loginPageData{
		CSRFToken: csrfToken,
	}

	h.render(w, "login.html", data)
}

func (h *handler) handlePostLogin(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	account, err := h.users.ByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			h.renderLoginError(w, r)
			return
		}

		h.logger.Error("failed to get user", "error", err)

		http.Error(w, "failed to sign in", http.StatusInternalServerError)
		return
	}

	valid, err := auth.VerifyPassword(password, account.PasswordHash)
	if err != nil {
		h.logger.Error("failed to verify password", "user_id", account.ID, "error", err)

		http.Error(w, "failed to sign in", http.StatusInternalServerError)
		return
	}

	if !valid || account.Disabled {
		h.renderLoginError(w, r)
		return
	}

	token, err := h.sessions.Create(r.Context(), account.ID)
	if err != nil {
		h.logger.Error("failed to create session", "user_id", account.ID, "error", err)

		http.Error(w, "failed to sign in", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, r, token)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handler) renderLoginError(w http.ResponseWriter, r *http.Request) {
	csrfToken, err := getCSRFToken(w, r)
	if err != nil {
		h.logger.Error("failed to get CSRF token", "error", err)

		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}

	data := loginPageData{
		Error:     "Invalid username or password",
		CSRFToken: csrfToken,
	}

	h.renderStatus(w, http.StatusUnauthorized, "login.html", data)
}

func (h *handler) handlePostLogout(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			clearSessionCookie(w, r)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		http.Error(w, "failed to sign out", http.StatusInternalServerError)
		return
	}

	if err := h.sessions.Delete(r.Context(), cookie.Value); err != nil {
		h.logger.Error("failed to delete session", "error", err)

		http.Error(w, "failed to sign out", http.StatusInternalServerError)
		return
	}

	clearSessionCookie(w, r)

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
