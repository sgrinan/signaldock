package web

import (
	"net/http"

	"github.com/sgrinan/signaldock/internal/user"
)

type usersPageData struct {
	Users     []user.User
	Error     string
	CSRFToken string
}

func (h *handler) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List()
	if err != nil {
		h.logger.Error("failed to list users", "error", err)

		http.Error(w, "failed to load users", http.StatusInternalServerError)
		return
	}

	csrfToken, err := getCSRFToken(w, r)
	if err != nil {
		h.logger.Error("failed to get CSRF token", "error", err)

		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}

	data := usersPageData{
		Users:     users,
		CSRFToken: csrfToken,
	}

	h.render(w, "users.html", data)
}
