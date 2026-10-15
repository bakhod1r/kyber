// Package seal encrypts secrets stored in the database (AES-256-GCM, ADR-0018) under
// KYBER_ENCRYPTION_KEY: 32 random bytes, base64.
package seal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// ErrOpen: the ciphertext is damaged or was sealed under another key.
var ErrOpen = errors.New("cannot decrypt the stored secret")

type Box struct{ aead cipher.AEAD }

// New returns nil (and no error) for an empty key: storing secrets is then disabled.
func New(key string) (*Box, error) {
	if key == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(raw) != 32 {
		return nil, fmt.Errorf("KYBER_ENCRYPTION_KEY must be 32 bytes in base64 (openssl rand -base64 32)")
	}
	block, _ := aes.NewCipher(raw) // a 32-byte key never fails
	aead, _ := cipher.NewGCM(block)
	return &Box{aead: aead}, nil
}

// Seal returns nonce || ciphertext.
func (b *Box) Seal(plain string) []byte {
	nonce := make([]byte, b.aead.NonceSize())
	_, _ = rand.Read(nonce) // never fails (crypto/rand, Go 1.24+)
	return b.aead.Seal(nonce, nonce, []byte(plain), nil)
}

func (b *Box) Open(sealed []byte) (string, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return "", ErrOpen
	}
	plain, err := b.aead.Open(nil, sealed[:n], sealed[n:], nil)
	if err != nil {
		return "", ErrOpen
	}
	return string(plain), nil
}
