// Package auth issues and verifies HMAC-signed bearer tokens that bind a
// userID to an expiry, replacing the spoofable userId query parameter.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMalformedToken = errors.New("malformed token")
	ErrBadSignature   = errors.New("invalid token signature")
	ErrExpiredToken   = errors.New("token expired")
)

// Authenticator signs and verifies tokens of the form
// base64url(userID|expiryUnix).base64url(HMAC-SHA256(payload)).
// It is safe for concurrent use.
type Authenticator struct {
	secret []byte
	now    func() time.Time
}

// New returns an Authenticator keyed by secret.
func New(secret string) *Authenticator {
	return &Authenticator{secret: []byte(secret), now: time.Now}
}

// Sign returns a token authenticating userID until now+ttl.
func (a *Authenticator) Sign(userID string, ttl time.Duration) string {
	payload := userID + "|" + strconv.FormatInt(a.now().Add(ttl).Unix(), 10)
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(payload)) + "." + enc.EncodeToString(a.sign([]byte(payload)))
}

// Verify validates the token's signature and expiry and returns the
// authenticated userID.
func (a *Authenticator) Verify(token string) (string, error) {
	enc := base64.RawURLEncoding
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", ErrMalformedToken
	}

	payload, err := enc.DecodeString(parts[0])
	if err != nil {
		return "", ErrMalformedToken
	}
	sig, err := enc.DecodeString(parts[1])
	if err != nil {
		return "", ErrMalformedToken
	}

	// Constant-time comparison guards against signature timing attacks.
	if !hmac.Equal(sig, a.sign(payload)) {
		return "", ErrBadSignature
	}

	fields := strings.SplitN(string(payload), "|", 2)
	if len(fields) != 2 || fields[0] == "" {
		return "", ErrMalformedToken
	}
	exp, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return "", ErrMalformedToken
	}
	if a.now().Unix() > exp {
		return "", ErrExpiredToken
	}
	return fields[0], nil
}

func (a *Authenticator) sign(payload []byte) []byte {
	m := hmac.New(sha256.New, a.secret)
	m.Write(payload)
	return m.Sum(nil)
}
