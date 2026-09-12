package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func newGrafanaProxy(grafanaURL string, logger *slog.Logger) (http.Handler, error) {
	target, err := url.Parse(grafanaURL)
	if err != nil {
		return nil, fmt.Errorf("parse Grafana URL: %w", err)
	}

	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("unsupported Grafana URL scheme %q", target.Scheme)
	}

	if target.Host == "" {
		return nil, fmt.Errorf("Grafana URL host is required")
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		logger.Error("Grafana proxy failed", "error", err)

		http.Error(w, "Grafana unavailable", http.StatusBadGateway)
	}

	return proxy, nil
}
