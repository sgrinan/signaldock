package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
)

const (
	csrfCookieName = "csrf_token"
	csrfFormField  = "csrf_token"
)

func generateCSRFToken() (string, error) {
	key := make([]byte, 32)

	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate CSRF token: %w", err)
	}

	return hex.EncodeToString(key), nil
}

func getCSRFToken(w http.ResponseWriter, r *http.Request) (string, error) {
	cookie, err := r.Cookie(csrfCookieName)
	if err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	if err != nil && err != http.ErrNoCookie {
		return "", fmt.Errorf("read CSRF cookie: %w", err)
	}

	token, err := generateCSRFToken()
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
	})

	return token, nil
}

func validateCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}

	formToken := r.PostFormValue(csrfFormField)
	if formToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare(
		[]byte(cookie.Value),
		[]byte(formToken),
	) == 1
}
