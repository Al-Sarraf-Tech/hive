package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims represents JWT payload claims.
type Claims struct {
	UserID   string `json:"sub"`
	Username string `json:"username"`
	Role     string `json:"role"`
	IssuedAt int64  `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("invalid token")
)

// jwtHeader is the fixed header for all tokens (alg: HS256).
var jwtHeaderB64 = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// GenerateToken creates a signed JWT with the given claims and TTL.
func GenerateToken(secret []byte, userID, username, role string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := jwtHeaderB64 + "." + payloadB64

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + sig, nil
}

// ValidateToken verifies a JWT signature and expiration.
func ValidateToken(secret []byte, tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrTokenInvalid
	}

	signingInput := parts[0] + "." + parts[1]

	// Verify signature
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, ErrTokenInvalid
	}

	// Decode claims
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenInvalid
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrTokenInvalid
	}

	// Check expiration
	if time.Now().UTC().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}
