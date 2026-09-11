package session

import "testing"

func TestGenerateToken(t *testing.T) {
	token := generateToken()

	if token == "" {
		t.Error("generateToken() = empty string, want non-empty token")
	}
}

func TestGenerateTokenUnique(t *testing.T) {
	first := generateToken()
	second := generateToken()

	if first == second {
		t.Errorf("generateToken() returned identical tokens, want different tokens")
	}
}

func TestHashToken(t *testing.T) {
	token := "test-token"

	first := hashToken(token)
	second := hashToken(token)

	if first != second {
		t.Errorf("hashToken(%q) = %q and %q, want identical hashes", token, first, second)
	}

	if first == "" {
		t.Errorf("hashToken(%q) = empty string, want non-empty hash", token)
	}
}

func TestHashTokenDifferentTokens(t *testing.T) {
	first := hashToken("first-token")
	second := hashToken("second-token")

	if first == second {
		t.Errorf(
			"hashToken() returned identical hashes for different tokens, want different hashes",
		)
	}
}
