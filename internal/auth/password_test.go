package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "correct horse battery staple"

	hash := HashPassword(password)

	if hash == "" {
		t.Fatal("HashPassword() = empty string, want non-empty hash")
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("HashPassword() = %q, want argon2id hash", hash)
	}
}

func TestHashPasswordRandomSalt(t *testing.T) {
	password := "same-password"

	first := HashPassword(password)
	second := HashPassword(password)

	if first == second {
		t.Errorf("HashPassword(%q) returned identical hashes, want different hashes", password)
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "correct-password"
	hash := HashPassword(password)

	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{
			name:     "correct_password",
			password: "correct-password",
			want:     true,
		},
		{
			name:     "wrong_password",
			password: "wrong-password",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyPassword(tt.password, hash)
			if err != nil {
				t.Fatalf("VerifyPassword(%q, hash) error = %v, want nil", tt.password, err)
			}

			if got != tt.want {
				t.Errorf("VerifyPassword(%q, hash) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}

func TestVerifyPasswordInvalidHash(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{
			name: "malformed",
			hash: "not-a-valid-hash",
		},
		{
			name: "unsupported_algorithm",
			hash: "$argon2i$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		},
		{
			name: "unsupported_version",
			hash: "$argon2id$v=999$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyPassword("password", tt.hash)

			if err == nil {
				t.Errorf("VerifyPassword(%q, %q) returned nil error, want error", "password", tt.hash)
			}
		})
	}
}
