package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func generateToken() string {
	return rand.Text()
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))

	return hex.EncodeToString(hash[:])
}
