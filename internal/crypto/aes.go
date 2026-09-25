package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

const aesKeySize = 32 // AES-256

// AESProvider encrypts with AES-256-GCM. Ciphertext is the random nonce
// followed by the sealed data.
type AESProvider struct {
	aead cipher.AEAD
}

func generateAESKey() ([]byte, error) {
	key := make([]byte, aesKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

func newAES(key []byte) (*AESProvider, error) {
	if len(key) != aesKeySize {
		return nil, fmt.Errorf("invalid aes key: expected %d bytes, got %d", aesKeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESProvider{aead: aead}, nil
}

func (p *AESProvider) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, p.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return p.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (p *AESProvider) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := p.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := p.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("aes decrypt (wrong key or corrupted file): %w", err)
	}
	return plaintext, nil
}
