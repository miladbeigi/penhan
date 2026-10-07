package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/miladbeigi/penhan/internal/access"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var accessCmd = &cobra.Command{
	Use:   "access",
	Short: "Share the project master key through GitHub SSH keys",
	Long: `The master key encrypts each safe's key, so wrapped safe keys can be
committed (see penhan wrap). The master key itself is encrypted to the SSH
keys of everyone granted access, fetched from https://github.com/<user>.keys
and pinned in .penhan/access.yaml. Anyone with one of those SSH private keys
runs penhan unlock to get the master key on their machine.

Run these commands at the project root, where penhan add creates safes.`,
}

var accessGrantCmd = &cobra.Command{
	Use:   "grant <github-user>",
	Short: "Give a GitHub user access to the master key",
	Long: `Grant fetches the user's SSH keys from GitHub, pins their fingerprints in
.penhan/access.yaml, and re-encrypts .penhan/master.age to every pinned key.
The first grant creates the master key.

Running it again for a user picks up keys they added or removed on GitHub; the
change is shown and must be confirmed (or pass --yes). Keys added on GitHub
never get access until someone runs grant.`,
	Args: cobra.ExactArgs(1),
	RunE: runAccessGrant,
}

var accessRevokeCmd = &cobra.Command{
	Use:   "revoke <github-user>",
	Short: "Remove a user and rotate the master key",
	Long: `Revoke removes the user from .penhan/access.yaml, creates a new master key,
re-wraps every safe key with it, and re-encrypts master.age to the remaining
keys. The old master key can't unwrap anything committed afterwards.

The revoked user may still have copies of the safe keys and the secrets
themselves. To fully cut them off, rotate those secrets as well.`,
	Args: cobra.ExactArgs(1),
	RunE: runAccessRevoke,
}

var accessListCmd = &cobra.Command{
	Use:   "list",
	Short: "List who has access and their pinned keys",
	Args:  cobra.NoArgs,
	RunE:  runAccessList,
}

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Decrypt the master key with your SSH key",
	Long: `Unlock decrypts .penhan/master.age with one of your SSH private keys and
stores the master key in the gitignored .penhan/master.key. After that, every
safe with a wrapped key works without its plaintext key.

It looks at the key pairs in ~/.ssh (a private key with its .pub next to it)
and only reads the private key the file was encrypted to. Pass --identity for
keys elsewhere. Keys held only by ssh-agent or a security key (sk-*) can't be
used.`,
	Args: cobra.NoArgs,
	RunE: runUnlock,
}

func init() {
	accessGrantCmd.Flags().StringSlice("key", nil, "Only grant these key fingerprints (SHA256:...), repeatable")
	accessGrantCmd.Flags().BoolP("yes", "y", false, "Accept changes to the user's pinned keys without asking")
	unlockCmd.Flags().StringSlice("identity", nil, "SSH private key to use, repeatable (default: key pairs in ~/.ssh)")
	accessCmd.AddCommand(accessGrantCmd, accessRevokeCmd, accessListCmd)
	rootCmd.AddCommand(accessCmd, unlockCmd)
}

// fetchKeys fetches a user's SSH keys; tests replace it.
var fetchKeys = access.FetchGitHubKeys

// projectRoot finds the project with a master key, starting at the current
// directory.
func projectRoot() (string, error) {
	root, err := access.FindRoot(".")
	if err != nil {
		return "", err
	}
	if root == "" {
		return "", fmt.Errorf("no %s found here or above; create one with `penhan access grant <github-user>` at the project root", filepath.Join(access.Dir, access.AccessFile))
	}
	return root, nil
}

func runAccessGrant(cmd *cobra.Command, args []string) error {
	user := args[0]
	onlyKeys, _ := cmd.Flags().GetStringSlice("key")
	yes, _ := cmd.Flags().GetBool("yes")

	root, err := access.FindRoot(".")
	if err != nil {
		return err
	}
	var f *access.File
	var master []byte
	created := root == ""
	if created {
		if _, err := os.Stat("penhan.yaml"); err == nil {
			return errors.New("this is a safe; run grant at the project root, where you ran penhan add")
		}
		if root, err = filepath.Abs("."); err != nil {
			return err
		}
		if master, err = crypto.GenerateMasterKey(); err != nil {
			return err
		}
		f = &access.File{MasterCheck: access.Check(master), Users: map[string][]access.Key{}}
	} else {
		if f, err = access.Load(root); err != nil {
			return err
		}
		if master, err = requireMaster(root); err != nil {
			return err
		}
	}

	keys, skipped, err := fetchKeys(cmd.Context(), user)
	if err != nil {
		return err
	}
	for _, s := range skipped {
		fmt.Fprintf(os.Stderr, "skipping a key of %s: %s\n", user, s)
	}
	if len(onlyKeys) > 0 {
		keys, err = selectKeys(keys, onlyKeys, user)
		if err != nil {
			return err
		}
	}

	old, existed := f.Users[user]
	added, removed := diffKeys(old, keys)
	if existed && len(added) == 0 && len(removed) == 0 {
		fmt.Printf("· %s already has access with the same %d key(s)\n", user, len(keys))
		return nil
	}
	if existed {
		fmt.Printf("%s's keys changed on GitHub:\n", user)
		for _, k := range added {
			fmt.Printf("  + %s\n", describeKey(k))
		}
		for _, k := range removed {
			fmt.Printf("  - %s\n", describeKey(k))
		}
		if len(removed) > 0 {
			fmt.Println("Removed keys can still unlock older commits of master.age; run `penhan access revoke` and grant again to rotate the master key.")
		}
		if !yes {
			if err := confirm(fmt.Sprintf("Update %s's pinned keys?", user)); err != nil {
				return err
			}
		}
	}

	f.Users[user] = keys
	sealed, err := access.Seal(master, f)
	if err != nil {
		return err
	}
	if created {
		if err := access.WriteMaster(root, master); err != nil {
			return err
		}
	}
	if err := os.WriteFile(access.Path(root, access.SealedFile), sealed, 0o644); err != nil {
		return err
	}
	if err := access.Save(root, f); err != nil {
		return err
	}
	if err := ignoreMasterKey(root); err != nil {
		return err
	}

	if created {
		fmt.Printf("✓ Created master key %s (gitignored)\n", access.Path(root, access.MasterFile))
	}
	fmt.Printf("✓ Granted %s access with %d key(s):\n", user, len(keys))
	for _, k := range keys {
		fmt.Printf("    %s\n", describeKey(k))
	}
	fmt.Printf("Commit %s and %s.\n", filepath.Join(access.Dir, access.AccessFile), filepath.Join(access.Dir, access.SealedFile))
	if created {
		fmt.Println("Next: run `penhan wrap` here to wrap the existing safe keys.")
	}
	return nil
}

func runAccessRevoke(cmd *cobra.Command, args []string) error {
	user := args[0]
	root, err := projectRoot()
	if err != nil {
		return err
	}
	f, err := access.Load(root)
	if err != nil {
		return err
	}
	if _, ok := f.Users[user]; !ok {
		return fmt.Errorf("%s has no access", user)
	}
	if len(f.Users) == 1 {
		return fmt.Errorf("%s is the only user with access; grant someone else first", user)
	}
	oldMaster, err := requireMaster(root)
	if err != nil {
		return err
	}
	newMaster, err := crypto.GenerateMasterKey()
	if err != nil {
		return err
	}

	// Re-wrap everything in memory first, so a safe that can't be unwrapped
	// stops the revoke before anything is written.
	safes, err := findSafes(root)
	if err != nil {
		return err
	}
	rewrapped := map[string][]byte{}
	for _, safe := range safes {
		cfg, err := config.Load(filepath.Join(safe, "penhan.yaml"))
		if err != nil {
			return fmt.Errorf("%s: %w", safe, err)
		}
		p := filepath.Join(safe, wrappedKeyPath(cfg.Encryption.Method))
		wrapped, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		key, err := crypto.UnwrapKey(oldMaster, wrapped)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if rewrapped[p], err = crypto.WrapKey(newMaster, key); err != nil {
			return err
		}
	}

	delete(f.Users, user)
	f.MasterCheck = access.Check(newMaster)
	sealed, err := access.Seal(newMaster, f)
	if err != nil {
		return err
	}
	for p, wrapped := range rewrapped {
		if err := os.WriteFile(p, wrapped, 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(access.Path(root, access.SealedFile), sealed, 0o644); err != nil {
		return err
	}
	if err := access.Save(root, f); err != nil {
		return err
	}
	if err := access.WriteMaster(root, newMaster); err != nil {
		return err
	}

	fmt.Printf("✓ Revoked %s and rotated the master key (%d safe key(s) re-wrapped)\n", user, len(rewrapped))
	fmt.Println("Commit the changes. Everyone else runs `penhan unlock` again after pulling.")
	fmt.Fprintf(os.Stderr, "\033[33mWarning: %s may still have copies of the safe keys and secrets. Rotate those secrets to fully cut off access.\033[0m\n", user)
	return nil
}

func runAccessList(cmd *cobra.Command, args []string) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	f, err := access.Load(root)
	if err != nil {
		return err
	}
	local := localFingerprints()
	for _, user := range f.SortedUsers() {
		fmt.Println(user)
		for _, k := range f.Users[user] {
			mark := ""
			if local[k.Fingerprint] {
				mark = "  (this machine)"
			}
			fmt.Printf("    %s%s\n", describeKey(k), mark)
		}
	}
	return nil
}

func runUnlock(cmd *cobra.Command, args []string) error {
	identityPaths, _ := cmd.Flags().GetStringSlice("identity")
	root, err := projectRoot()
	if err != nil {
		return err
	}
	f, err := access.Load(root)
	if err != nil {
		return err
	}
	sealed, err := os.ReadFile(access.Path(root, access.SealedFile))
	if err != nil {
		return err
	}
	var passphrase access.PassphraseFunc
	if stdinIsTTY() {
		passphrase = func(path string) ([]byte, error) {
			fmt.Fprintf(os.Stderr, "Passphrase for %s: ", path)
			defer fmt.Fprintln(os.Stderr)
			return term.ReadPassword(os.Stdin.Fd())
		}
	}
	ids, err := access.LocalIdentities(identityPaths, passphrase)
	if err != nil {
		return err
	}
	master, err := access.Unseal(sealed, ids)
	if err != nil {
		return err
	}
	if err := f.VerifyMaster(master); err != nil {
		return fmt.Errorf("master.age and access.yaml disagree about the master key: %w", err)
	}
	if err := access.WriteMaster(root, master); err != nil {
		return err
	}
	if err := ignoreMasterKey(root); err != nil {
		return err
	}
	fmt.Printf("✓ Unlocked the master key into %s (gitignored)\n", access.Path(root, access.MasterFile))
	return nil
}

// ignoreMasterKey makes sure the unlocked master key is gitignored.
func ignoreMasterKey(root string) error {
	return appendGitignoreAt(filepath.Join(root, ".gitignore"), []string{access.Dir + "/" + access.MasterFile})
}

func selectKeys(keys []access.Key, fingerprints []string, user string) ([]access.Key, error) {
	var out []access.Key
	for _, fp := range fingerprints {
		i := slices.IndexFunc(keys, func(k access.Key) bool { return k.Fingerprint == fp })
		if i < 0 {
			return nil, fmt.Errorf("%s has no usable key %s on GitHub", user, fp)
		}
		out = append(out, keys[i])
	}
	return out, nil
}

// diffKeys compares two key sets by fingerprint.
func diffKeys(old, cur []access.Key) (added, removed []access.Key) {
	has := func(ks []access.Key, fp string) bool {
		return slices.ContainsFunc(ks, func(k access.Key) bool { return k.Fingerprint == fp })
	}
	for _, k := range cur {
		if !has(old, k.Fingerprint) {
			added = append(added, k)
		}
	}
	for _, k := range old {
		if !has(cur, k.Fingerprint) {
			removed = append(removed, k)
		}
	}
	return added, removed
}

func describeKey(k access.Key) string {
	typ, _, _ := strings.Cut(k.Key, " ")
	return k.Fingerprint + " " + typ
}

// localFingerprints returns the fingerprints of the public keys in ~/.ssh.
func localFingerprints() map[string]bool {
	out := map[string]bool{}
	home, err := os.UserHomeDir()
	if err != nil {
		return out
	}
	pubs, _ := filepath.Glob(filepath.Join(home, ".ssh", "*.pub"))
	for _, p := range pubs {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if pk, _, _, _, err := ssh.ParseAuthorizedKey(data); err == nil {
			out[ssh.FingerprintSHA256(pk)] = true
		}
	}
	return out
}

// confirm asks a yes/no question, refusing in non-interactive runs.
func confirm(question string) error {
	if !stdinIsTTY() {
		return errors.New("confirmation needed; pass --yes in non-interactive runs")
	}
	ok := false
	if err := huh.NewConfirm().Title(question).Value(&ok).Run(); err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}
	return nil
}
