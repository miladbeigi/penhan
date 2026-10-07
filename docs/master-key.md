# Sharing keys with a master key

Without a master key, every safe's key lives only on the machine that created it, and moving it to another machine means copying it by hand. A master key removes that step:

- The **master key** encrypts each safe's key. The encrypted copy, `<safe>/.penhan/aes.key.enc`, is committed.
- The master key itself is encrypted to the **SSH public keys of everyone with access**, taken from `https://github.com/<user>.keys`, and committed as `.penhan/master.age`.
- On a new machine, `penhan unlock` decrypts the master key with your SSH private key. From then on every safe works from a plain clone.

```
your SSH key ──unlocks──▶ master key ──unwraps──▶ safe keys ──decrypt──▶ secrets/*.enc
 (~/.ssh)               (.penhan/master.age)    (<safe>/.penhan/*.key.enc)
```

Everything committed is encrypted. The plaintext master key is kept in `.penhan/master.key` at the project root, readable only by you, and gitignored.

## Set it up

At the project root, where you run `penhan add`:

```bash
penhan access grant <your-github-user>   # creates the master key, encrypts it to your GitHub SSH keys
penhan wrap                              # wraps the key of every existing safe
git add .penhan/access.yaml .penhan/master.age */.penhan/*.key.enc
git commit -m "Share safe keys through the master key"
```

`grant` encrypts to **all** your SSH keys on GitHub, so any of your machines can unlock it. Pass `--key SHA256:...` (repeatable) to limit it to specific keys.

`wrap` needs the plaintext key of each safe it wraps. At the root it skips safes whose key is on another machine; run `penhan unlock` and `penhan wrap` there too.

Once the master key exists, `penhan add` wraps new safe keys automatically. If the master key isn't unlocked on that machine, `add` stops and asks you to run `penhan unlock` first, so new safes can't be created with keys that aren't shared.

## On another machine

```bash
git pull
penhan unlock    # anywhere in the project
cd myapp && penhan check
```

`unlock` looks at the key pairs in `~/.ssh` (a private key with its `.pub` next to it) and only opens the private key that `master.age` was encrypted to, so you're asked for at most one passphrase. Use `--identity <path>` for keys elsewhere.

When a safe has both a plaintext key in `.penhan/keys/` and a wrapped key, the plaintext key is used.

## Who has access

```bash
penhan access list                    # users and pinned key fingerprints
penhan access grant <github-user>     # add someone, or pick up key changes
penhan access revoke <github-user>    # remove someone and rotate the master key
```

Granting needs the master key to be unlocked on your machine.

Each user's keys are **pinned** in `.penhan/access.yaml` when they're granted. Keys a user adds on GitHub later get no access until someone runs `grant` again. When a re-grant finds added or removed keys, it shows the change and asks before applying it (`--yes` to skip the question in scripts). Review changes to `access.yaml` in pull requests like any other access change.

`revoke` creates a new master key, re-wraps every safe key with it, and re-encrypts `master.age` to the remaining users. Everyone else runs `penhan unlock` again after pulling. Revoking can't take back what someone already saw: they may have copies of the safe keys and the secret values. To fully cut off access, rotate those secrets too.

## CI

A pipeline has no SSH key. Give it the master key in `PENHAN_MASTER_KEY`, the contents of `.penhan/master.key`:

```bash
PENHAN_MASTER_KEY=$(cat .penhan/master.key)   # store this as a CI secret
```

## Supported SSH keys

| Key type | Supported |
|---|---|
| `ssh-ed25519` | Yes |
| `ssh-rsa` | Yes |
| `ecdsa-sha2-*` | No. `grant` skips these keys with a warning. |
| `sk-ssh-ed25519@openssh.com`, `sk-ecdsa-…` (security keys) | No. A security key can sign but not decrypt. |

`unlock` reads the private key file itself. A key that only exists inside `ssh-agent` or a password manager's SSH agent can't be used, because the agent protocol can sign but not decrypt.

## Files

| Path | Committed | Contents |
|---|---|---|
| `.penhan/access.yaml` | Yes | Users, their pinned SSH public keys, and a check value that identifies the current master key |
| `.penhan/master.age` | Yes | The master key, encrypted with [age](https://age-encryption.org) to every pinned key |
| `.penhan/master.key` | No | The unlocked master key |
| `<safe>/.penhan/<method>.key.enc` | Yes | The safe's key, encrypted with the master key (AES-256-GCM) |
