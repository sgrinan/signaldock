package session

import "testing"

func TestGenerateToken(t *testing.T) {
	token := GenerateToken()

	if token == "" {
		t.Error("GenerateToken() = empty string, want non-empty token")
	}
}

func TestGenerateTokenUnique(t *testing.T) {
	first := GenerateToken()
	second := GenerateToken()

	if first == second {
		t.Errorf("GenerateToken() returned identical tokens, want different tokens")
	}
}

func TestHashToken(t *testing.T) {
	token := "test-token"

	first := HashToken(token)
	second := HashToken(token)

	if first != second {
		t.Errorf("HashToken(%q) = %q and %q, want identical hashes", token, first, second)
	}

	if first == "" {
		t.Errorf("HashToken(%q) = empty string, want non-empty hash", token)
	}
}

func TestHashTokenDifferentTokens(t *testing.T) {
	first := HashToken("first-token")
	second := HashToken("second-token")

	if first == second {
		t.Errorf(
			"HashToken() returned identical hashes for different tokens, want different hashes",
		)
	}
}
