package auth

import (
	"strings"
	"testing"
	"time"
)

func TestAuthenticator_RoundTrip(t *testing.T) {
	a := New("super-secret")
	token := a.Sign("user-123", time.Hour)

	userID, err := a.Verify(token)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if userID != "user-123" {
		t.Errorf("expected user-123, got %q", userID)
	}
}

func TestAuthenticator_RejectsTamperedSignature(t *testing.T) {
	a := New("super-secret")
	token := a.Sign("user-123", time.Hour)

	tampered := token[:len(token)-2] + "xx"
	if _, err := a.Verify(tampered); err == nil {
		t.Error("expected tampered token to be rejected")
	}
}

func TestAuthenticator_RejectsWrongSecret(t *testing.T) {
	token := New("secret-a").Sign("user-123", time.Hour)
	if _, err := New("secret-b").Verify(token); err != ErrBadSignature {
		t.Errorf("expected ErrBadSignature, got %v", err)
	}
}

func TestAuthenticator_RejectsExpired(t *testing.T) {
	a := New("super-secret")
	base := time.Now()
	a.now = func() time.Time { return base }
	token := a.Sign("user-123", time.Minute)

	// Jump past expiry.
	a.now = func() time.Time { return base.Add(2 * time.Minute) }
	if _, err := a.Verify(token); err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestAuthenticator_RejectsMalformed(t *testing.T) {
	a := New("super-secret")
	for _, tok := range []string{"", "no-dot", "a.b.c", strings.Repeat("x", 10)} {
		if _, err := a.Verify(tok); err == nil {
			t.Errorf("expected malformed token %q to be rejected", tok)
		}
	}
}
