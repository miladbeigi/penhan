package backends

import "errors"

// Supported backend types, as written in penhan.yaml.
const (
	TypeVault      = "vault"
	TypeFile       = "file"
	TypeKubernetes = "kubernetes"
)

// ErrNotFound is returned by Pull when the backend holds no secret at the
// requested path. Callers use errors.Is to tell "new secret" from a real failure.
var ErrNotFound = errors.New("secret not found")

// Encryptor handles encryption and decryption of data at rest.
// Satisfied by crypto.Provider implementations.
type Encryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// Provider is a secret store penhan pushes to. Content is always a JSON
// object of string key-value pairs; path is the secret's path under the
// secrets directory without extension, e.g. "db/password".
type Provider interface {
	// Push writes content at path, replacing whatever is stored there.
	Push(content []byte, path string) error

	// Pull reads the content stored at path. It returns an error wrapping
	// ErrNotFound when nothing is stored there.
	Pull(path string) ([]byte, error)
}
