package auth

import (
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword("secret123", hash) {
		t.Error("expected password to match")
	}
	if CheckPassword("wrong", hash) {
		t.Error("expected wrong password to not match")
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	cfg := NewConfig("test-secret-key")

	token, err := cfg.GenerateToken("alice", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}

	claims, err := cfg.ValidateToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Username != "alice" {
		t.Errorf("expected username alice, got %s", claims.Username)
	}
	if claims.Role != "admin" {
		t.Errorf("expected role admin, got %s", claims.Role)
	}
}

func TestValidateTokenInvalid(t *testing.T) {
	cfg := NewConfig("test-secret-key")

	_, err := cfg.ValidateToken("garbage")
	if err == nil {
		t.Error("expected error for invalid token")
	}

	// Token signed with different secret
	other := NewConfig("other-secret")
	token, _ := other.GenerateToken("bob", "viewer")
	_, err = cfg.ValidateToken(token)
	if err == nil {
		t.Error("expected error for token signed with different secret")
	}
}

func TestRoleAtLeast(t *testing.T) {
	tests := []struct {
		user, required string
		want           bool
	}{
		{"admin", "admin", true},
		{"admin", "editor", true},
		{"admin", "viewer", true},
		{"editor", "editor", true},
		{"editor", "viewer", true},
		{"editor", "admin", false},
		{"viewer", "viewer", true},
		{"viewer", "editor", false},
		{"viewer", "admin", false},
		{"unknown", "viewer", false},
	}
	for _, tt := range tests {
		got := RoleAtLeast(tt.user, tt.required)
		if got != tt.want {
			t.Errorf("RoleAtLeast(%q, %q) = %v, want %v", tt.user, tt.required, got, tt.want)
		}
	}
}

func TestNewConfigRandomSecret(t *testing.T) {
	cfg := NewConfig("")
	if len(cfg.JWTSecret) != 32 {
		t.Errorf("expected 32-byte random secret, got %d bytes", len(cfg.JWTSecret))
	}
}

func TestGenerateWebhookSecret(t *testing.T) {
	s := GenerateWebhookSecret()
	if len(s) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("expected 32 hex chars, got %d", len(s))
	}
}
