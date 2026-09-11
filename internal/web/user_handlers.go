package web

import (
	"errors"
	"net/http"
	"strings"

	"uuid"

	"github.com/sgrinan/signaldock/internal/auth"
	"github.com/sgrinan/signaldock/internal/user"
)

type usersPageData struct {
	Users       []user.User
	Error       string
	CSRFToken   string
	CurrentUser user.User
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

	account, ok := currentUser(r)
	if !ok {
		http.Error(w, "failed to get current user", http.StatusInternalServerError)
		return
	}

	data := usersPageData{
		Users:       users,
		CSRFToken:   csrfToken,
		CurrentUser: account,
	}

	h.render(w, "users.html", data)
}

func (h *handler) handlePostUser(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	role := user.Role(r.FormValue("role"))

	if username == "" || password == "" {
		h.renderUsersError(w, r, "Username and password are required")
		return
	}

	if role != user.RoleAdmin && role != user.RoleViewer {
		h.renderUsersError(w, r, "Invalid user role")
		return
	}

	account := user.User{
		ID:           uuid.NewV7(),
		Username:     username,
		PasswordHash: auth.HashPassword(password),
		Role:         role,
	}

	if err := h.users.Insert(account); err != nil {
		if errors.Is(err, user.ErrUserExists) {
			h.renderUsersError(w, r, "Username already exists")
			return
		}

		h.logger.Error("failed to create user", "error", err)

		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (h *handler) renderUsersError(w http.ResponseWriter, r *http.Request, message string) {
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

	account, ok := currentUser(r)
	if !ok {
		http.Error(w, "failed to get current user", http.StatusInternalServerError)
		return
	}

	data := usersPageData{
		Users:       users,
		Error:       message,
		CSRFToken:   csrfToken,
		CurrentUser: account,
	}

	h.renderStatus(w, http.StatusBadRequest, "users.html", data)
}

func (h *handler) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	rawID := r.PathValue("id")

	id, err := uuid.Parse(rawID)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	account, ok := currentUser(r)
	if !ok {
		http.Error(w, "failed to get current user", http.StatusInternalServerError)
		return
	}

	if id == account.ID {
		http.Error(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}

	if err := h.users.RemoveByID(id); err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		h.logger.Error("failed to delete user", "user_id", id, "error", err)

		http.Error(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (h *handler) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	current, ok := currentUser(r)
	if !ok {
		http.Error(w, "failed to get current user", http.StatusInternalServerError)
		return
	}

	if id == current.ID {
		http.Error(w, "cannot modify your own account", http.StatusBadRequest)
		return
	}

	role := user.Role(r.FormValue("role"))
	if role != user.RoleAdmin && role != user.RoleViewer {
		http.Error(w, "invalid user role", http.StatusBadRequest)
		return
	}

	disabled := r.FormValue("disabled") == "true"

	if err := h.users.SetRole(id, role); err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		h.logger.Error("failed to update user role", "user_id", id, "error", err)
		http.Error(w, "failed to update user", http.StatusInternalServerError)
		return
	}

	if err := h.users.SetDisabled(id, disabled); err != nil {
		h.logger.Error("failed to update user status", "user_id", id, "error", err)
		http.Error(w, "failed to update user", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}
