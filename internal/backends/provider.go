package backends

import "errors"

// ErrNotFound is returned by Pull when the backend holds no secret at the
// requested path. Callers use errors.Is to tell "new secret" from a real failure.
var ErrNotFound = errors.New("secret not found")

// Encryptor handles encryption and decryption of data at rest.
// Satisfied by crypto.Provider implementations.
type Encryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// SetupOptions carries backend-specific configuration.
// Not all fields are used by every provider — each provider reads what it needs.
type SetupOptions struct {
	Addr      string
	Token     string
	MountPath string
	BasePath  string
	Dir       string
	Enc       Encryptor
}

// Provider defines the interface for backend storage providers.
// Implementations can target any secret storage service (Vault, file, etc.).
type Provider interface {
	// Push uploads secret content to the backend.
	Push(content []byte, remotePath string) error

	// Pull downloads secret content from the backend. It returns an error
	// wrapping ErrNotFound when nothing is stored at remotePath.
	Pull(remotePath string) ([]byte, error)

	// List returns all secret paths at the given remote path.
	List(remotePath string) ([]string, error)

	// Delete removes a secret from the backend.
	Delete(remotePath string) error

	// Setup initializes the provider with the given options.
	Setup(opts SetupOptions) error

	// IsInitialized returns true if the provider is ready to use.
	IsInitialized() bool
}
