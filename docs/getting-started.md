# Getting started

This guide walks through creating a safe, adding secrets, and syncing them to a backend. For install instructions, see the [README](../README.md#install).

## Concepts

- **Project**: any directory, usually a Git repository, that holds one or more safes.
- **Safe**: a subdirectory created by `penhan add`. It has its own configuration, encryption key, and backend settings. A common layout is one safe per application, environment, or Kubernetes namespace.
- **Secret**: a flat YAML or JSON file of key-value pairs under the safe's `secrets/` directory. Its path, minus the extension, is its name in the backend: `secrets/db/main.yaml` becomes `db/main`.

## Create a safe

Run `penhan add` from the project directory. Without flags, it asks for everything interactively:

```bash
penhan add myapp
```

In scripts and CI, pass every option as a flag. `add` fails immediately if one is missing rather than waiting for input:

```bash
penhan add myapp --encryption=aes --backend=vault \
  --vault-addr=https://vault.example.com --vault-token-file=./vault-token
```

This creates:

```
project/
├── .gitignore            # plaintext secrets, keys, and tokens are added here
└── myapp/
    ├── penhan.yaml       # safe configuration (committed)
    ├── secrets/          # secret files; only the .enc copies are committed
    └── .penhan/
        ├── keys/aes.key  # encryption key (gitignored, back it up)
        └── vault-token   # backend credential, Vault only (gitignored)
```

See [Vault](backends/vault.md), [Kubernetes](backends/kubernetes.md), or [File](backends/file.md) for backend-specific options.

## Add a secret

Secrets are plain files, so create them with any editor:

```bash
cd myapp
cat > secrets/db.yaml <<'YAML'
username: app
password: s3cr3t
YAML
```

Values are sent to the backend exactly as written: `pin: 0012` stays `0012`. Nested maps and lists are rejected.

## Encrypt, check, push

```bash
penhan encrypt   # writes secrets/db.yaml.enc and removes the plaintext
penhan check     # compares every local secret with the backend
penhan push      # writes the secrets reported as new or changed
git add -A && git commit -m "Add database credentials"
```

`check` never writes anything. It reports each secret as:

| Status | Meaning |
|---|---|
| `new` | Nothing is stored at this path in the backend |
| `changed` | The backend holds different content |
| `unchanged` | Identical, nothing to push |

The local files are the source of truth. An edit made directly in the backend shows up as `changed`, and the next `push` overwrites it. Only secrets that exist locally are considered: secrets that exist only in the backend are never reported or deleted.

## Edit a secret

```bash
penhan decrypt secrets/db.yaml.enc   # writes secrets/db.yaml; the .enc file stays
$EDITOR secrets/db.yaml
penhan encrypt                        # re-encrypts only what changed
penhan push
```

Secrets you decrypted but didn't change keep their `.enc` files byte for byte, so `git diff` shows only real changes. `check` and `push` read plaintext and `.enc` files alike; when both exist, the plaintext wins, since it's the copy being edited.

## Working as a team

The `.enc` files and `penhan.yaml` are committed; keys are not. Anyone who clones the repository needs a copy of each safe's key to work with it:

```bash
git clone git@github.com:acme/secrets.git && cd secrets/myapp
mkdir -p .penhan/keys
cp /path/from/password-manager/aes.key .penhan/keys/aes.key
penhan check
```

Without the key, every command stops with `encryption key not found` rather than creating a new key that couldn't decrypt the existing files. Share keys through a password manager or another channel outside Git. See [Encryption and keys](encryption.md).

For a Vault safe, each person also needs a token at `.penhan/vault-token`. For a Kubernetes safe, penhan uses their kubeconfig.

## Next steps

- [Commands](commands.md): every command and flag
- [Configuration](configuration.md): the `penhan.yaml` reference
- [Kubernetes backend](backends/kubernetes.md#importing-existing-secrets): bring existing cluster Secrets under management
