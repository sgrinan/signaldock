package web

import (
	"errors"
	"net/http"

	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

func (h *handler) handleGetIndex(w http.ResponseWriter, r *http.Request) {
	result := r.URL.Query().Get("result")

	csrfToken, err := getCSRFToken(w, r)
	if err != nil {
		h.logger.Error("failed to get CSRF token", "error", err)

		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}

	endpoints := h.endpoints.List()

	data := pageData{
		Endpoints: newEndpointListItems(endpoints),
		CSRFToken: csrfToken,
	}

	switch result {
	case "unreachable":
		data.Error = "Endpoint was added, but it is currently unreachable"

	case "duplicate":
		data.Error = "Endpoint already exists"

	case "invalid":
		data.Error = "Enter a valid HTTP or HTTPS URL"

	case "unsafe":
		data.Error = "Private or unsafe network destinations are not allowed"

	case "tls-invalid":
		data.Error = "Endpoint was added, but its TLS certificate is invalid"

	case "check-failed":
		data.Error = "Endpoint was added, but its HTTP and TLS checks failed"
	}

	h.render(w, "index.html", data)
}

func (h *handler) handlePostEndpoint(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	rawURL := r.PostFormValue("url")

	added, err := h.endpoints.Add(rawURL)

	var checkErr endpoint.CheckError

	switch {
	case err == nil:
		h.logger.Info("endpoint added", "id", added.ID)

		http.Redirect(w, r, "/", http.StatusSeeOther)

	case errors.Is(err, endpoint.ErrEndpointExists):
		http.Redirect(w, r, "/?result=duplicate", http.StatusSeeOther)

	case errors.Is(err, endpoint.ErrInvalidURL),
		errors.Is(err, endpoint.ErrUnsupportedScheme),
		errors.Is(err, endpoint.ErrHostRequired),
		errors.Is(err, endpoint.ErrCredentialsNotAllowed),
		errors.Is(err, endpoint.ErrInvalidPort):

		http.Redirect(w, r, "/?result=invalid", http.StatusSeeOther)

	case errors.Is(err, endpoint.ErrUnsafeHost):
		http.Redirect(w, r, "/?result=unsafe", http.StatusSeeOther)

	case errors.As(err, &checkErr):
		switch {

		case checkErr.TLS != nil &&
			added.LastCheck.TLS.Enabled &&
			!added.LastCheck.TLS.Valid &&
			!added.LastCheck.TLS.ExpiresAt.IsZero():

			h.logger.Warn("endpoint TLS certificate validation failed", "id", added.ID, "error", checkErr.TLS)

			http.Redirect(w, r, "/?result=tls-invalid", http.StatusSeeOther)
			return

		case checkErr.Host != nil:
			h.logger.Warn("endpoint host check failed", "id", added.ID, "error", checkErr.Host)

			http.Redirect(w, r, "/?result=unreachable", http.StatusSeeOther)

		case checkErr.HTTP != nil && checkErr.TLS != nil:
			h.logger.Warn("endpoint HTTP and TLS checks failed", "id", added.ID, "http_error", checkErr.HTTP, "tls_error", checkErr.TLS)

			http.Redirect(w, r, "/?result=check-failed", http.StatusSeeOther)

		case checkErr.HTTP != nil:
			h.logger.Warn("endpoint HTTP check failed", "id", added.ID, "error", checkErr.HTTP)

			http.Redirect(w, r, "/?result=unreachable", http.StatusSeeOther)

		case checkErr.TLS != nil:
			h.logger.Warn("endpoint TLS check failed", "id", added.ID, "error", checkErr.TLS)

			http.Redirect(w, r, "/?result=tls-invalid", http.StatusSeeOther)

		default:
			h.logger.Error("endpoint check failed without error details", "id", added.ID)

			http.Error(w, "failed to check endpoint", http.StatusInternalServerError)
		}

	default:
		h.logger.Error("failed to add endpoint", "error", err)

		http.Error(w, "failed to add endpoint", http.StatusInternalServerError)
	}
}

func (h *handler) handleGetEndpoint(w http.ResponseWriter, r *http.Request) {
	rawID := r.PathValue("id")

	id, err := uuid.Parse(rawID)
	if err != nil {
		http.Error(w, "invalid endpoint ID", http.StatusBadRequest)
		return
	}

	ep, err := h.endpoints.ByID(id)
	if err != nil {
		if errors.Is(err, endpoint.ErrEndpointNotFound) {
			http.Error(w, "endpoint not found", http.StatusNotFound)
			return
		}

		h.logger.Error("failed to get endpoint", "id", id, "error", err)

		http.Error(w, "failed to get endpoint", http.StatusInternalServerError)
		return
	}

	csrfToken, err := getCSRFToken(w, r)
	if err != nil {
		h.logger.Error("failed to get CSRF token", "endpoint_id", id, "error", err)

		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}

	// Reuse the same endpoint state shown on the dashboard.
	item := newEndpointListItem(ep)

	tlsExpiresAt := ""
	tlsDaysClass := ""

	if !ep.LastCheck.TLS.ExpiresAt.IsZero() {
		tlsExpiresAt = ep.LastCheck.TLS.ExpiresAt.Format("02 Jan 2006")
		tlsDaysClass = tlsExpiryClass(ep.LastCheck.TLS.DaysRemaining)
	}

	lastCheckedAt := ""

	if !ep.LastCheck.HTTP.CheckedAt.IsZero() {
		lastCheckedAt = ep.LastCheck.HTTP.CheckedAt.Format("15:04:05")
	}

	data := endpointPageData{
		Endpoint:      ep,
		CSRFToken:     csrfToken,
		LatencyMS:     ep.LastCheck.HTTP.Latency.Milliseconds(),
		LastCheckedAt: lastCheckedAt,
		TLSExpiresAt:  tlsExpiresAt,
		State:         item.State,
		StateClass:    item.StateClass,
		TLSDaysClass:  tlsDaysClass,
		Checking:      ep.LastCheck.HTTP.CheckedAt.IsZero(),
	}

	h.render(w, "endpoint.html", data)
}

func (h *handler) handleDeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	rawID := r.PathValue("id")

	id, err := uuid.Parse(rawID)
	if err != nil {
		http.Error(w, "invalid endpoint ID", http.StatusBadRequest)
		return
	}

	if err := h.endpoints.RemoveByID(id); err != nil {
		if errors.Is(err, endpoint.ErrEndpointNotFound) {
			http.Error(w, "endpoint not found", http.StatusNotFound)
			return
		}

		h.logger.Error("failed to remove endpoint", "id", id, "error", err)

		http.Error(w, "failed to remove endpoint", http.StatusInternalServerError)
		return
	}

	h.logger.Info("endpoint removed", "id", id)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handler) handleRefreshEndpoint(w http.ResponseWriter, r *http.Request) {
	if !validateCSRF(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	rawID := r.PathValue("id")

	id, err := uuid.Parse(rawID)
	if err != nil {
		http.Error(w, "invalid endpoint ID", http.StatusBadRequest)
		return
	}

	check, err := h.endpoints.Refresh(id)

	var checkErr endpoint.CheckError

	switch {
	case err == nil:
		h.logger.Info("endpoint refreshed", "id", id, "status_code", check.HTTP.StatusCode, "responded", check.HTTP.Responded,
			"latency", check.HTTP.Latency, "tls_enabled", check.TLS.Enabled, "tls_valid", check.TLS.Valid)

	case errors.Is(err, endpoint.ErrEndpointNotFound):
		http.Error(w, "endpoint not found", http.StatusNotFound)
		return

	case errors.As(err, &checkErr):
		h.logger.Warn("endpoint refresh completed with check errors", "id", id, "host_error", checkErr.Host, "http_error", checkErr.HTTP, "tls_error", checkErr.TLS)

	default:
		h.logger.Error("endpoint refresh failed", "id", id, "error", err)

		http.Error(w, "failed to refresh endpoint", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/endpoints/"+id.String(), http.StatusSeeOther)
}
