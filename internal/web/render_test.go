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
	t.Run("success", func(t *testing.T) {
		templates := template.Must(
			template.New("page").Parse(`hello {{.}}`),
		)

		h := &handler{
			templates: templates,
			logger: slog.New(
				slog.NewTextHandler(io.Discard, nil),
			),
		}

		recorder := httptest.NewRecorder()

		h.render(recorder, "page", "SignalDock")

		if got, want := recorder.Code, http.StatusOK; got != want {
			t.Errorf("render() status = %d, want %d", got, want)
		}

		if got, want := recorder.Body.String(), "hello SignalDock"; got != want {
			t.Errorf("render() body = %q, want %q", got, want)
		}

		if got, want := recorder.Header().Get("Content-Type"), "text/html; charset=utf-8"; got != want {
			t.Errorf("render() Content-Type = %q, want %q", got, want)
		}
	})

	t.Run("template_error", func(t *testing.T) {
		templates := template.Must(
			template.New("page").Parse(`{{.Missing}}`),
		)

		h := &handler{
			templates: templates,
			logger: slog.New(
				slog.NewTextHandler(io.Discard, nil),
			),
		}

		recorder := httptest.NewRecorder()

		h.render(recorder, "page", struct{}{})

		if got, want := recorder.Code, http.StatusInternalServerError; got != want {
			t.Errorf("render() status = %d, want %d", got, want)
		}

		if got, want := recorder.Body.String(), "failed to render page\n"; got != want {
			t.Errorf("render() body = %q, want %q", got, want)
		}
	})
}
