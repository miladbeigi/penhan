package crypto

import (
	"errors"
	"fmt"
	"os"
)

// Supported encryption methods, as written in penhan.yaml.
const (
	MethodGPG = "gpg"
	MethodAES = "aes"
)

// ErrKeyNotFound is returned by Load when the key file does not exist.
var ErrKeyNotFound = errors.New("encryption key not found")

// Provider encrypts and decrypts secret files with a safe's key.
type Provider interface {
	// Encrypt encrypts plaintext bytes and returns ciphertext.
	Encrypt(plaintext []byte) ([]byte, error)

	// Decrypt decrypts ciphertext bytes and returns plaintext.
	Decrypt(ciphertext []byte) ([]byte, error)
}

// IsMethod reports whether method is a supported encryption method.
func IsMethod(method string) bool {
	return method == MethodGPG || method == MethodAES
}

// Generate creates a new key for method at keyPath. It never overwrites an
// existing key: replacing a key makes every file encrypted with it unreadable.
func Generate(method, keyPath string) error {
	var key []byte
	var err error
	switch method {
	case MethodGPG:
		key, err = generateGPGKey()
	case MethodAES:
		key, err = generateAESKey()
	default:
		return unsupported(method)
	}
	if err != nil {
		return fmt.Errorf("generate %s key: %w", method, err)
	}
	return writeKey(keyPath, key)
}

// Load reads the key for method from keyPath. It never creates a key, so a
// missing key is reported instead of silently replaced with a new one.
func Load(method, keyPath string) (Provider, error) {
	if !IsMethod(method) {
		return nil, unsupported(method)
	}
	if keyPath == "" {
		return nil, fmt.Errorf("%s key path not configured", method)
	}
	key, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w at %s; copy this safe's key from whoever created it", ErrKeyNotFound, keyPath)
	}
	if err != nil {
		return nil, fmt.Errorf("read %s key: %w", method, err)
	}
	if method == MethodGPG {
		return newGPG(key)
	}
	return newAES(key)
}

func unsupported(method string) error {
	if method == "github-gpg" {
		return fmt.Errorf("encryption method github-gpg is no longer supported; decrypt the .enc files with `gpg --decrypt` and recreate the safe with gpg or aes")
	}
	return fmt.Errorf("unsupported encryption method: %q (must be gpg or aes)", method)
}

// writeKey writes a private key readable only by the owner, failing if a
// file already exists at path.
func writeKey(path string, key []byte) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("key already exists at %s; refusing to overwrite it", path)
	}
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	_, err = f.Write(key)
	return err
}
