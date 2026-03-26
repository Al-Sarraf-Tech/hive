// Package secrets provides encrypted secret storage using age encryption.
package secrets

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"
)

// Vault encrypts and decrypts secrets using an age X25519 identity.
type Vault struct {
	identity  *age.X25519Identity
	recipient *age.X25519Recipient
}

// NewVault loads or creates an age keypair in the given data directory.
// The keypair file is `hive-key.txt` inside dataDir.
func NewVault(dataDir string) (*Vault, error) {
	keyPath := filepath.Join(dataDir, "hive-key.txt")

	data, err := os.ReadFile(keyPath)
	if err == nil {
		// Parse existing identity
		identity, err := age.ParseX25519Identity(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse age identity from %s: %w", keyPath, err)
		}
		return &Vault{
			identity:  identity,
			recipient: identity.Recipient(),
		}, nil
	}

	// Generate new identity
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, fmt.Errorf("generate age identity: %w", err)
	}

	// Write private key (restrictive permissions)
	if err := os.WriteFile(keyPath, []byte(identity.String()), 0o600); err != nil {
		return nil, fmt.Errorf("write age identity to %s: %w", keyPath, err)
	}

	return &Vault{
		identity:  identity,
		recipient: identity.Recipient(),
	}, nil
}

// Encrypt encrypts plaintext bytes using the local age public key.
func (v *Vault) Encrypt(plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, v.recipient)
	if err != nil {
		return nil, fmt.Errorf("create age writer: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("write plaintext: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close age writer: %w", err)
	}
	return buf.Bytes(), nil
}

// Decrypt decrypts age-encrypted bytes using the local identity.
func (v *Vault) Decrypt(ciphertext []byte) ([]byte, error) {
	r, err := age.Decrypt(bytes.NewReader(ciphertext), v.identity)
	if err != nil {
		return nil, fmt.Errorf("age decrypt: %w", err)
	}
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read decrypted: %w", err)
	}
	return plaintext, nil
}

// PublicKey returns the age public key (for sharing with peers).
func (v *Vault) PublicKey() string {
	return v.recipient.String()
}
