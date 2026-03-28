package joincode

import (
	"strings"
	"testing"
)

func TestEncodeFormat(t *testing.T) {
	code, err := Encode("test-token-abc123")
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	// Must match HIVE-XXXX-XXXX
	if !strings.HasPrefix(code, "HIVE-") {
		t.Errorf("expected HIVE- prefix, got %q", code)
	}
	parts := strings.Split(code, "-")
	if len(parts) != 3 {
		t.Fatalf("expected 3 dash-separated parts, got %d: %q", len(parts), code)
	}
	if len(parts[1]) != 4 || len(parts[2]) != 4 {
		t.Errorf("expected 4+4 char groups, got %d+%d: %q", len(parts[1]), len(parts[2]), code)
	}
}

func TestEncodeDeterministic(t *testing.T) {
	a, _ := Encode("same-token")
	b, _ := Encode("same-token")
	if a != b {
		t.Errorf("Encode not deterministic: %q vs %q", a, b)
	}
}

func TestEncodeDifferentTokens(t *testing.T) {
	a, _ := Encode("token-alpha")
	b, _ := Encode("token-beta")
	if a == b {
		t.Errorf("different tokens produced same code: %q", a)
	}
}

func TestEncodeEmptyToken(t *testing.T) {
	_, err := Encode("")
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestDecodeValid(t *testing.T) {
	code, _ := Encode("my-token")
	normalized, err := Decode(code)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(normalized) != 8 {
		t.Errorf("expected 8-char normalized code, got %d: %q", len(normalized), normalized)
	}
}

func TestDecodeCaseInsensitive(t *testing.T) {
	a, _ := Decode("HIVE-AB12-CD34")
	b, _ := Decode("hive-ab12-cd34")
	if a != b {
		t.Errorf("decode not case-insensitive: %q vs %q", a, b)
	}
}

func TestDecodeInvalidLength(t *testing.T) {
	_, err := Decode("HIVE-ABC")
	if err == nil {
		t.Error("expected error for short code")
	}
}

func TestDecodeConfusableChars(t *testing.T) {
	// Crockford base32 corrects I->1, L->1, O->0
	result, err := Decode("HIVE-IOLL-IOLL")
	if err != nil {
		t.Fatalf("expected confusable correction, got error: %v", err)
	}
	if result != "10111011" {
		t.Errorf("expected 10111011, got %s", result)
	}
}

func TestDecodeInvalidChars(t *testing.T) {
	// 'U' is not in Crockford base32 and not a confusable
	_, err := Decode("HIVE-UUUU-UUUU")
	if err == nil {
		t.Error("expected error for invalid characters")
	}
}

func TestMatchesTrue(t *testing.T) {
	token := "secret-join-token-xyz"
	code, _ := Encode(token)
	if !Matches(token, code) {
		t.Error("Matches returned false for matching token/code")
	}
}

func TestMatchesFalse(t *testing.T) {
	code, _ := Encode("correct-token")
	if Matches("wrong-token", code) {
		t.Error("Matches returned true for non-matching token")
	}
}

func TestMatchesNormalization(t *testing.T) {
	token := "my-token"
	code, _ := Encode(token)
	// Lowercase version should still match
	lower := strings.ToLower(code)
	if !Matches(token, lower) {
		t.Error("Matches should be case-insensitive")
	}
}

func TestAllCharsInAlphabet(t *testing.T) {
	// Encode many tokens and ensure all produced chars are in the alphabet
	for i := range 100 {
		token := strings.Repeat("t", i+1)
		code, err := Encode(token)
		if err != nil {
			t.Fatalf("Encode(%q) failed: %v", token, err)
		}
		normalized, err := Decode(code)
		if err != nil {
			t.Fatalf("Decode(%q) failed: %v", code, err)
		}
		for _, c := range normalized {
			if !strings.ContainsRune(alphabet, c) {
				t.Errorf("character %q not in Crockford base32 alphabet (code %q)", c, code)
			}
		}
	}
}

func TestTokenPrefix(t *testing.T) {
	prefix := TokenPrefix("my-token")
	if len(prefix) != 8 { // 4 bytes = 8 hex chars
		t.Errorf("expected 8 hex chars, got %d: %q", len(prefix), prefix)
	}
}
