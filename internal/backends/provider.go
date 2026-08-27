package backends

// Provider defines the interface for backend storage providers.
type Provider interface {
	// Push uploads local secrets to the backend.
	Push(localPath string, remotePath string) error

	// Pull downloads secrets from the backend to local.
	Pull(remotePath string, localPath string) error

	// List returns all secret paths at the given remote path.
	List(remotePath string) ([]string, error)

	// Delete removes a secret from the backend.
	Delete(remotePath string) error

	// Setup initializes the provider with the given config.
	Setup(addr string, token string, mountPath string, basePath string) error

	// IsInitialized returns true if the provider is ready to use.
	IsInitialized() bool
}
