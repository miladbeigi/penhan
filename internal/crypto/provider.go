package crypto

// Provider defines the interface for encryption providers.
type Provider interface {
	// Encrypt encrypts plaintext bytes and returns ciphertext.
	Encrypt(plaintext []byte) ([]byte, error)

	// Decrypt decrypts ciphertext bytes and returns plaintext.
	Decrypt(ciphertext []byte) ([]byte, error)

	// Setup initializes the provider with the given config.
	Setup(keyPath string, passphrase string) error

	// IsInitialized returns true if the provider is ready to use.
	IsInitialized() bool
}
