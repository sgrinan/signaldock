package web

import (
	"bytes"
	"net/http"
)

func (h *handler) render(w http.ResponseWriter, templateName string, data any) {
	var buf bytes.Buffer

	if err := h.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		h.logger.Error("failed to render template", "template", templateName, "error", err)

		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if _, err := buf.WriteTo(w); err != nil {
		h.logger.Error("failed to write response", "template", templateName, "error", err)
	}
}
