package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateToken returns a cryptographically random session token.
func GenerateToken() string {
	return rand.Text()
}

// HashToken returns the SHA-256 hash used to persist token securely.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}
