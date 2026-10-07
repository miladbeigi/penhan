package crypto

import "fmt"

// MasterKeySize is the size of a project master key: an AES-256 key.
const MasterKeySize = aesKeySize

// GenerateMasterKey returns a new random master key.
func GenerateMasterKey() ([]byte, error) {
	return generateAESKey()
}

// WrapKey encrypts a safe key with the master key, so the result can be
// committed. Like every AES encryption here it is randomized.
func WrapKey(master, key []byte) ([]byte, error) {
	p, err := newAES(master)
	if err != nil {
		return nil, fmt.Errorf("master key: %w", err)
	}
	return p.Encrypt(key)
}

// UnwrapKey decrypts a safe key wrapped with WrapKey.
func UnwrapKey(master, wrapped []byte) ([]byte, error) {
	p, err := newAES(master)
	if err != nil {
		return nil, fmt.Errorf("master key: %w", err)
	}
	key, err := p.Decrypt(wrapped)
	if err != nil {
		return nil, fmt.Errorf("unwrap safe key (wrong master key?): %w", err)
	}
	return key, nil
}
