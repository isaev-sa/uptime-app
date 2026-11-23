package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNormalize(t *testing.T) {
	for _, tc := range []struct {
		email, password, want string
		valid                 bool
	}{
		{" User@Example.COM ", "password123", "user@example.com", true},
		{"not-email", "password123", "", false}, {"u@example.com", "short", "", false},
	} {
		got, err := normalize(tc.email, tc.password)
		if (err == nil) != tc.valid || got != tc.want {
			t.Errorf("normalize(%q): %q, %v", tc.email, got, err)
		}
	}
}
func TestAccessTokens(t *testing.T) {
	s := Service{Secret: []byte("01234567890123456789012345678901"), AccessTTL: time.Minute, RefreshTTL: time.Hour}
	tokens, _, err := s.issue("user-id", "user@example.com", "User", "")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.VerifyAccess(tokens.Access)
	if err != nil || id != "user-id" {
		t.Fatalf("verify: %s %v", id, err)
	}
	other := s
	other.Secret = []byte("other secret")
	if _, err = other.VerifyAccess(tokens.Access); err == nil {
		t.Fatal("accepted wrong signature")
	}
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "user-id", ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))})
	raw, err := expired.SignedString(s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.VerifyAccess(raw); err == nil {
		t.Fatal("accepted expired token")
	}
}
