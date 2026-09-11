package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	memory      uint32 = 19 * 1024
	iterations  uint32 = 2
	parallelism uint8  = 1
	saltLength         = 16
	keyLength          = 32
)

// HashPassword returns an Argon2id hash of password using a random salt.
func HashPassword(password string) string {
	salt := make([]byte, saltLength)
	rand.Read(salt)

	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memory, iterations, parallelism, b64Salt, b64Hash)

	return encodedHash
}

// VerifyPassword reports whether password matches encodedHash.
// It returns an error if encodedHash is malformed or unsupported.
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")

	if len(parts) != 6 {
		return false, fmt.Errorf("invalid password hash format")
	}

	if parts[1] != "argon2id" {
		return false, fmt.Errorf("unsupported password hash algorithm")
	}

	var version int

	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse argon2 version: %w", err)
	}

	if version != argon2.Version {
		return false, fmt.Errorf("unsupported argon2 version")
	}

	var (
		hashMemory      uint32
		hashIterations  uint32
		hashParallelism uint8
	)

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &hashMemory, &hashIterations, &hashParallelism); err != nil {
		return false, fmt.Errorf("parse argon2 parameters: %w", err)
	}

	if hashMemory != memory ||
		hashIterations != iterations ||
		hashParallelism != parallelism {
		return false, fmt.Errorf("unsupported argon2 parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode password salt: %w", err)
	}

	if len(salt) != saltLength {
		return false, fmt.Errorf("invalid password salt length")
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode password hash: %w", err)
	}

	if len(expectedHash) != keyLength {
		return false, fmt.Errorf("invalid password hash length")
	}

	actualHash := argon2.IDKey([]byte(password), salt, hashIterations, hashMemory, hashParallelism, keyLength)

	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1, nil
}
