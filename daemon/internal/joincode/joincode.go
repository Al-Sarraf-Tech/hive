// Package joincode encodes and decodes short human-readable join codes
// in the format HIVE-XXXX-XXXX. The code is derived from the SHA-256 of
// the cluster join token (first 5 bytes → 8 Crockford base32 chars) and
// acts as a lookup key — the init node stores the full mapping.
package joincode

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// Crockford base32 alphabet — no I, L, O, U to avoid ambiguity.
const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Encode creates a short join code from a token.
// Format: HIVE-XXXX-XXXX (8 Crockford base32 chars derived from SHA-256).
func Encode(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("token must not be empty")
	}
	h := sha256.Sum256([]byte(token))
	// Pack first 5 bytes (40 bits) into a uint64 for easy bit extraction.
	bits := uint64(h[0])<<32 | uint64(h[1])<<24 | uint64(h[2])<<16 | uint64(h[3])<<8 | uint64(h[4])
	var code strings.Builder
	for i := 0; i < 8; i++ {
		idx := (bits >> (35 - uint(i)*5)) & 0x1f
		code.WriteByte(alphabet[idx])
	}
	result := code.String()
	return fmt.Sprintf("HIVE-%s-%s", result[:4], result[4:]), nil
}

// Decode normalizes a join code string, stripping the "HIVE-" prefix and
// dashes. Returns the canonical 8-char uppercase code or an error.
func Decode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.TrimPrefix(code, "HIVE-")
	code = strings.ReplaceAll(code, "-", "")
	// Crockford base32 confusable corrections
	code = strings.ReplaceAll(code, "I", "1")
	code = strings.ReplaceAll(code, "L", "1")
	code = strings.ReplaceAll(code, "O", "0")
	if len(code) != 8 {
		return "", fmt.Errorf("invalid join code format: expected 8 characters, got %d", len(code))
	}
	for _, c := range code {
		if !strings.ContainsRune(alphabet, c) {
			return "", fmt.Errorf("invalid character %q in join code", c)
		}
	}
	return code, nil
}

// Matches returns true if the given token produces the same join code.
func Matches(token, code string) bool {
	encoded, err := Encode(token)
	if err != nil {
		return false
	}
	a, err := Decode(encoded)
	if err != nil {
		return false
	}
	b, err := Decode(code)
	if err != nil {
		return false
	}
	return a == b
}

// TokenPrefix returns the first 4 bytes of SHA-256(token) as hex.
// Useful for logging without exposing the full token.
func TokenPrefix(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h[:4])
}
