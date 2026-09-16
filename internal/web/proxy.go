package web

import (
	"context"
	"net/http"
	"strings"
)

type secureRequestContextKey struct{}

func trustProxyHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedProto := r.Header.Get("X-Forwarded-Proto")
		proto, _, _ := strings.Cut(forwardedProto, ",")

		if strings.EqualFold(strings.TrimSpace(proto), "https") {
			ctx := context.WithValue(r.Context(), secureRequestContextKey{}, true)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

func requestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	secure, _ := r.Context().Value(secureRequestContextKey{}).(bool)
	return secure
}
