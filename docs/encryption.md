# Encryption and keys

## Methods

Each safe uses one method, chosen when it's created:

| Method | Algorithm | Key file |
|---|---|---|
| `aes` | AES-256-GCM with a random 256-bit key | `.penhan/keys/aes.key` |
| `gpg` | OpenPGP, with a keypair generated for the safe | `.penhan/keys/gpg.key` |

Both keys are generated locally by `penhan add`, readable only by you (mode `0600`), and never leave the machine unless you copy them. There's nothing to fetch and no external key service.

## What is committed

`penhan add` adds these entries to the project's `.gitignore` for each safe:

```gitignore
myapp/secrets/**/*.yaml
myapp/secrets/**/*.yml
myapp/secrets/**/*.json
myapp/.penhan/keys/
myapp/.penhan/vault-token      # Vault safes only
```

So plaintext secrets at any depth, keys, and tokens stay out of Git. What gets committed is `penhan.yaml` and the encrypted `*.enc` files.

> Safes created with penhan 0.5.x or earlier have entries like `myapp/secrets/*.yaml`, which don't match nested files such as `secrets/db/main.yaml`. Change them to the `**/` form above.

## Keys

- **Back up every key.** Anyone who clones the repository needs the key to decrypt the `.enc` files. If a key is lost, those files can't be recovered.
- **Share keys outside Git**, for example through a password manager.
- **Only `penhan add` creates keys.** If a key is missing, every other command stops with `encryption key not found at …` instead of generating a new one. A new key couldn't read the existing files, and files encrypted with it couldn't be read by the rest of the team.
- `add` never overwrites an existing key.

## Clean diffs

Encryption is randomized: AES uses a fresh nonce and GPG a fresh session key every time. Encrypting the same plaintext twice gives different bytes. This keeps anyone reading the Git history from telling when two secrets are equal, or when a value changes back to an old one.

To keep diffs meaningful anyway, `decrypt` leaves the `.enc` file in place, and `encrypt` keeps it unchanged when it already decrypts to the plaintext. Decrypting and re-encrypting without edits changes nothing in Git.

## Secret files

- Files are flat maps of keys to values, in YAML (`.yaml`, `.yml`) or JSON (`.json`). Nested maps and lists are rejected.
- Values are read exactly as written. YAML's type conversions don't apply: `0012`, `0x1F`, `1.10`, and `2026-01-02` all stay as written, and an empty value is an empty string.
- Duplicate keys are rejected.

## Migrating from `github-gpg`

penhan 0.6.0 removed the `github-gpg` method. A safe still configured with it fails with an explanation. To migrate, decrypt its `.enc` files with your GPG private key (`gpg --decrypt`) and create a new safe with `aes` or `gpg`.
