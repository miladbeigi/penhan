package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/miladbeigi/penhan/internal/access"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/spf13/cobra"
)

var wrapCmd = &cobra.Command{
	Use:   "wrap",
	Short: "Encrypt safe keys with the master key so they can be committed",
	Long: `Wrap encrypts a safe's key with the project master key and writes it to
.penhan/<method>.key.enc in the safe, which is committed. Anyone who can unlock
the master key can then use the safe without copying its key by hand.

Inside a safe it wraps that safe. At the project root it wraps every safe that
isn't wrapped yet. Safes created with penhan add after the master key exists
are wrapped automatically.`,
	Args: cobra.NoArgs,
	RunE: runWrap,
}

func init() {
	rootCmd.AddCommand(wrapCmd)
}

// wrappedKeyPath is where a safe's wrapped key lives, relative to the safe.
// It's outside .penhan/keys/, which is gitignored.
func wrappedKeyPath(method string) string {
	return filepath.Join(".penhan", method+".key.enc")
}

// requireMaster loads the project master key, failing when there is none.
func requireMaster(start string) ([]byte, error) {
	master, err := access.LoadMaster(start)
	if err != nil {
		return nil, err
	}
	if master == nil {
		return nil, errors.New("this project has no master key; create one with `penhan access grant <github-user>` at the project root")
	}
	return master, nil
}

func unwrapSafeKey(safeDir string, wrapped []byte) ([]byte, error) {
	master, err := requireMaster(safeDir)
	if err != nil {
		return nil, err
	}
	return crypto.UnwrapKey(master, wrapped)
}

func runWrap(cmd *cobra.Command, args []string) error {
	var safes []string
	if _, err := os.Stat("penhan.yaml"); err == nil {
		safes = []string{"."}
	} else {
		found, err := findSafes(".")
		if err != nil {
			return err
		}
		if len(found) == 0 {
			return errors.New("no safes found; run wrap inside a safe or at the project root")
		}
		safes = found
	}
	master, err := requireMaster(".")
	if err != nil {
		return err
	}
	for _, safe := range safes {
		msg, err := wrapSafe(safe, master)
		if err != nil {
			return fmt.Errorf("%s: %w", safe, err)
		}
		fmt.Printf("%s %s\n", msg, safe)
	}
	return nil
}

// wrapSafe writes the wrapped key of the safe in dir. An existing wrapped key
// is kept as long as it holds the same key.
func wrapSafe(dir string, master []byte) (string, error) {
	cfg, err := config.Load(filepath.Join(dir, "penhan.yaml"))
	if err != nil {
		return "", err
	}
	method := cfg.Encryption.Method
	wrappedPath := filepath.Join(dir, wrappedKeyPath(method))
	key, err := os.ReadFile(filepath.Join(dir, cfg.Encryption.KeyPath()))
	if errors.Is(err, os.ErrNotExist) {
		if _, serr := os.Stat(wrappedPath); serr == nil {
			return "· already wrapped (no local key)", nil
		}
		return "", fmt.Errorf("%w at %s; wrap it on the machine that has the key", crypto.ErrKeyNotFound, cfg.Encryption.KeyPath())
	}
	if err != nil {
		return "", err
	}
	if _, err := crypto.New(method, key); err != nil {
		return "", err
	}

	if existing, err := os.ReadFile(wrappedPath); err == nil {
		old, err := crypto.UnwrapKey(master, existing)
		if err != nil {
			return "", err
		}
		if !bytes.Equal(old, key) {
			return "", fmt.Errorf("%s holds a different key than %s; remove one of them", wrappedPath, cfg.Encryption.KeyPath())
		}
		return "· already wrapped", nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	wrapped, err := crypto.WrapKey(master, key)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(wrappedPath, wrapped, 0o644); err != nil {
		return "", err
	}
	return "✓ wrapped", nil
}

// findSafes returns the directories under root holding a penhan.yaml. It
// doesn't descend into safes or hidden directories.
func findSafes(root string) ([]string, error) {
	var safes []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if p != root && d.Name()[0] == '.' {
			return filepath.SkipDir
		}
		if _, err := os.Stat(filepath.Join(p, "penhan.yaml")); err == nil {
			if p != root {
				safes = append(safes, p)
			}
			return filepath.SkipDir
		}
		return nil
	})
	sort.Strings(safes)
	return safes, err
}
