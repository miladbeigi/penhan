package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

type AESProvider struct {
	key []byte
}

func NewAESProvider() *AESProvider {
	return &AESProvider{}
}

func (p *AESProvider) Setup(keyPath, passphrase string) error {
	if passphrase != "" {
		// Fixed salt is acceptable for a local tool - it's a domain separator, not a secret
		salt := []byte("penhan-salt")
		p.key = pbkdf2.Key([]byte(passphrase), salt, 600000, 32, sha256.New)
		return nil
	}

	if keyPath == "" {
		return fmt.Errorf("aes key path not configured")
	}

	// Load an existing key so encrypt/decrypt work across invocations;
	// generate one only when the key file does not exist yet.
	key, err := os.ReadFile(keyPath)
	switch {
	case err == nil:
		if len(key) != 32 {
			return fmt.Errorf("invalid aes key at %s: expected 32 bytes, got %d", keyPath, len(key))
		}
		p.key = key
		return nil
	case !os.IsNotExist(err):
		return err
	}

	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	p.key = key

	return os.WriteFile(keyPath, key, 0o600)
}

func (p *AESProvider) IsInitialized() bool {
	return len(p.key) > 0
}

func (p *AESProvider) SealOnly() bool {
	return false
}

func (p *AESProvider) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (p *AESProvider) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
