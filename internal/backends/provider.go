package backends

// Provider defines the interface for backend storage providers.
// Implementations can target any secret storage service (Vault, AWS Secrets Manager, etc.).
type Provider interface {
	// Push uploads secret content to the backend.
	Push(content []byte, remotePath string) error

	// Pull downloads secret content from the backend.
	Pull(remotePath string) ([]byte, error)

	// List returns all secret paths at the given remote path.
	List(remotePath string) ([]string, error)

	// Delete removes a secret from the backend.
	Delete(remotePath string) error

	// Setup initializes the provider with the given config.
	Setup(addr string, token string, mountPath string, basePath string) error

	// IsInitialized returns true if the provider is ready to use.
	IsInitialized() bool
}
