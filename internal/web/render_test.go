package web

import (
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_Render(t *testing.T) {
	templates := template.Must(
		template.New("page").Parse(`hello {{.}}`),
	)

	h := &handler{
		templates: templates,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	recorder := httptest.NewRecorder()

	h.render(recorder, "page", "SignalDock")

	if recorder.Code != http.StatusOK {
		t.Errorf("render() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if got, want := recorder.Body.String(), "hello SignalDock"; got != want {
		t.Errorf("render() body = %q, want %q", got, want)
	}

	if got, want := recorder.Header().Get("Content-Type"), "text/html; charset=utf-8"; got != want {
		t.Errorf("render() Content-Type = %q, want %q", got, want)
	}
}

func TestHandler_RenderTemplateError(t *testing.T) {
	templates := template.Must(
		template.New("page").Parse(`{{.Missing}}`),
	)

	h := &handler{
		templates: templates,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	recorder := httptest.NewRecorder()

	h.render(recorder, "page", struct{}{})

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("render() status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
