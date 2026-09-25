# Penhan

[![CI](https://github.com/miladbeigi/penhan/actions/workflows/ci.yml/badge.svg)](https://github.com/miladbeigi/penhan/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/miladbeigi/penhan)](https://github.com/miladbeigi/penhan/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Git-native secret management with encryption and backend sync.**

Penhan keeps secrets as encrypted files in Git and pushes them to a secret backend: HashiCorp Vault, Kubernetes Secrets, or an encrypted directory on disk. Git is the source of truth; the backend is where applications read from.

## Features

- **Git-native** — secrets live in Git as encrypted files, versioned and auditable
- **Encrypted at rest** — GPG/PGP or AES-256-GCM, with keys generated locally in each safe
- **Safes** — each safe is a directory with its own config, key, and backend base path
- **Hash-based sync** — `check` compares local files with the backend; `push` writes only what changed
- **Directory mapping** — the folder structure under `secrets/` maps to backend paths automatically
- **Few commands** — add, check, push, encrypt, decrypt, import, update, version

## Backends

| Backend | Status |
|---------|--------|
| HashiCorp Vault KV v2 | Supported |
| Kubernetes Secrets | Supported |
| Encrypted file directory | Supported |
| AWS Secrets Manager | Planned |
| GCP Secret Manager | Planned |
| Azure Key Vault | Planned |

## Install

### Binary (recommended)

Download a pre-built binary from the [latest release](https://github.com/miladbeigi/penhan/releases/latest):

```bash
# macOS (Apple Silicon)
curl -Lo penhan.tar.gz https://github.com/miladbeigi/penhan/releases/latest/download/penhan_*_darwin_arm64.tar.gz
tar xzf penhan.tar.gz
sudo mv penhan /usr/local/bin/

# Linux (amd64)
curl -Lo penhan.tar.gz https://github.com/miladbeigi/penhan/releases/latest/download/penhan_*_linux_amd64.tar.gz
tar xzf penhan.tar.gz
sudo mv penhan /usr/local/bin/
```

### Go Install

Requires Go 1.26+:

```bash
go install github.com/miladbeigi/penhan/cmd/penhan@latest
```

### Updating

```bash
penhan update           # download, verify, and install the latest release
penhan update --check   # only report whether a newer release exists
```

`update` downloads the release archive for your OS and architecture, verifies it against the release's `checksums.txt`, runs the new binary once to make sure it works, and then atomically replaces the installed one. It never downgrades: a build newer than the latest release (e.g. one built from `master`) is left alone, even with `--force`. If penhan lives in a directory you can't write to (e.g. `/usr/local/bin`), run it with `sudo`.

When you use penhan in a terminal, it checks for a new release at most once a day and prints a one-line notice on stderr. It never installs anything by itself. The check is skipped in scripts and CI (when stderr isn't a terminal or `CI` is set); set `PENHAN_NO_UPDATE_NOTIFIER=1` to turn it off entirely.

### Build from Source

```bash
git clone https://github.com/miladbeigi/penhan.git
cd penhan
make build
```

## Quick Start

```bash
# Create a safe (interactive), or pass everything as flags
penhan add myapp
penhan add myapp \
  --encryption=aes \
  --backend=vault \
  --vault-addr=https://vault.example.com \
  --vault-token-file=./vault-token

# Or push to Kubernetes Secrets in a namespace (uses your kubeconfig)
penhan add myapp --encryption=aes --backend=kubernetes --kube-namespace=myapp

cd myapp

# Create a secret file: a flat YAML or JSON key-value map
mkdir -p secrets/db
echo "password: hunter2" > secrets/db/password.yaml

# Encrypt it in place for committing to Git
penhan encrypt

# See what differs from the backend
penhan check

# Push new and changed secrets to the backend
penhan push
```

## Commands

| Command | Description |
|---------|-------------|
| `penhan add [name]` | Create a safe: a subdirectory with its own config, key, and backend settings |
| `penhan check` | Compare local secrets with the backend and report `new`, `changed`, or `unchanged`. Never writes |
| `penhan push` | Push secrets whose hash differs from the backend; skip the rest. Prints every secret |
| `penhan encrypt [file\|dir]` | Encrypt secret files in place (defaults to the secrets directory) |
| `penhan decrypt [file\|dir]` | Write the plaintext next to each `.enc` file for editing (defaults to the secrets directory). Refuses to overwrite a plaintext file with different content |
| `penhan import [name...]` | Take over Secrets that already exist in the namespace (kubernetes backend). With no arguments, lists what can be imported |
| `penhan update` | Update penhan to the latest release (`--check` to only look) |
| `penhan version` | Print version information |

All commands except `add` and `version` run inside a safe directory.

## How It Works

### Safes

A project is a directory of safes. `penhan add <name>` creates one:

```
my-project/
├── .gitignore                 # add appends key, token, and plaintext patterns
└── myapp/                     # the safe
    ├── penhan.yaml            # safe configuration (committed)
    ├── secrets/               # secret files; the *.enc copies are committed
    │   ├── db/
    │   │   └── password.yaml.enc
    │   └── api/
    │       └── key.yaml.enc
    └── .penhan/               # keys and credentials (gitignored)
        ├── keys/
        │   └── aes.key        # key file named after the encryption method
        └── vault-token
```

Each safe sets its backend base path to its own name, so several safes can share one Vault without path collisions.

### Path Mapping

Local paths map to backend paths automatically:

| Local Path | Vault Path | Kubernetes Secret |
|------------|------------|-------------------|
| `secrets/db.yaml` | `secret/data/myapp/db` | `db` |
| `secrets/db/password.yaml` | `secret/data/myapp/db/password` | `db-password` |
| `secrets/api/key.yaml` | `secret/data/myapp/api/key` | `api-key` |

The Vault path is `{mount_path}/data/{base_path}/{secret path}`, where `base_path` is the safe name. The Kubernetes Secret name is the secret path with `/` replaced by `-`, in the safe's namespace.

### Kubernetes

The `kubernetes` backend writes each secret file as a native `Opaque` Secret, one data key per key in the file, so pods can mount it or read it with `envFrom` as usual. The `.enc` files in Git stay encrypted with the safe's key; penhan decrypts them locally and sends plaintext to the API server over TLS, just as it does for Vault. Encrypting Secrets at rest in etcd is the cluster's job: see [encryption at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).

Guardrails:

- **Context is pinned.** `add` records the kubeconfig context (the current one, or `--kube-context`) in `penhan.yaml`. Later pushes always go to that context, even after `kubectl config use-context`. If the context doesn't exist in someone's kubeconfig, penhan fails instead of falling back to another cluster.
- **Ownership is enforced.** Every Secret penhan writes has the label `app.kubernetes.io/managed-by: penhan` and the annotations `penhan/safe` and `penhan/path`. `check` and `push` refuse to touch a Secret created by something else, owned by another safe, or holding a different path that maps to the same name (e.g. `db/password.yaml` and `db-password.yaml`).
- **Names are validated, not rewritten.** Secret names must be lowercase DNS names (`a-z`, `0-9`, `-`, `.`). A file like `secrets/DB_Password.yaml` is rejected with an error, not silently renamed, because renaming could merge two files into one Secret.
- **Pushes replace data.** A key removed from the local file is removed from the Secret. Labels and annotations added by other tools are kept.

#### Importing existing Secrets

To bring Secrets you created by hand under penhan, run `penhan import` inside the safe. With no arguments it only lists: each Secret in the namespace is shown as `ready`, `present` (already in the safe), or `skip` with the reason. Then import by name, or everything ready with `--all`:

```bash
penhan import                 # read-only: what can be imported, and why not
penhan import api-key db      # or: penhan import --all
```

Each imported Secret is written straight to `secrets/<name>.yaml.enc`, encrypted, so the plaintext never touches disk. The Secret gets the penhan label and annotations; its data, type, and other labels are unchanged, so nothing that mounts it restarts. penhan re-reads the data just before marking the Secret and refuses if it changed during the import. It never imports non-`Opaque` Secrets (e.g. `kubernetes.io/dockerconfigjson` image pull secrets), Secrets managed by Helm, Argo CD, or another tool, Secrets with an owner reference, or Secrets holding binary values.

penhan needs `get`, `list`, `create`, and `update` on `secrets` in the namespace. The namespace must already exist. No credentials are stored in the safe: penhan uses your kubeconfig (`--kubeconfig`, `$KUBECONFIG`, or `~/.kube/config`).

### Encryption

Penhan supports two encryption methods. Both generate their key locally when the safe is created and store it under `.penhan/keys/` (gitignored), so there is nothing to fetch and nothing to keep in sync:

- **`gpg`** — an OpenPGP keypair generated for the safe
- **`aes`** — symmetric AES-256-GCM with a random 256-bit key

Back up the key file and share it with teammates out of band: anyone who clones the repository needs it to decrypt the `.enc` files, and losing it means losing access to them. Only `penhan add` creates keys. Every other command fails with `encryption key not found` when the key is missing, rather than generating a new one that would not match the existing files.

Encryption is randomized (a fresh nonce for AES, a fresh session key for GPG), so encrypting the same plaintext twice gives different bytes; this keeps an observer of the git history from telling when two secrets are equal. To keep git diffs meaningful anyway, `decrypt` leaves the `.enc` file in place and `encrypt` keeps it byte for byte when it already decrypts to the plaintext. A decrypt/encrypt cycle without edits changes nothing in git.

Secret files are flat YAML or JSON key-value pairs (`.yaml`, `.yml`, or `.json`); nested maps or lists are rejected. Values are pushed exactly as written: `pin: 0012` stays `0012`, and an empty value is an empty string. `check` and `push` read both plaintext and `.enc` files; when both exist for the same secret, the plaintext wins.

### Check and Push

There is no local state file. `check` hashes each local secret's content and reads the same path from the backend:

- **new** — the backend has nothing at this path
- **changed** — the backend content hashes differently
- **unchanged** — identical, nothing to do

`push` runs the same comparison and writes the `new` and `changed` secrets. The local file is the source of truth: an edit made directly in the backend shows up as `changed` and is overwritten on the next push. Run `check` first if you want to see that before it happens. Only secrets that exist locally are considered; secrets that live solely in the backend are neither reported nor deleted.

## Configuration

`penhan.yaml` inside a safe (generated by `penhan add`):

```yaml
encryption:
  method: aes          # gpg or aes
  aes:
    key_path: .penhan/keys/aes.key
  # gpg:
  #   key_path: .penhan/keys/gpg.key

backend:
  type: vault          # vault, kubernetes, or file
  vault:
    addr: https://vault.example.com
    token_path: .penhan/vault-token
    mount_path: secret
    base_path: myapp   # the safe name
  # kubernetes:
  #   context: prod-cluster   # pinned by add
  #   namespace: myapp
  #   safe: myapp             # recorded on each Secret for ownership checks
  #   kubeconfig: /path/to/kubeconfig   # optional; default $KUBECONFIG or ~/.kube/config
  # file:
  #   path: .penhan/remote   # encrypted copies written here instead of Vault

secrets:
  path: secrets/
  format: yaml
```

## Development

```bash
# Run unit tests
make test

# Run all CI checks
make ci

# Build binary
make build

# Run linter
make lint

# Run integration tests (requires Docker)
make test-integration

# Run e2e tests: real CLI against throwaway Vault and k3s containers (requires Docker)
make test-e2e
```

## Release Process

Releases are automated via GitHub Actions and GoReleaser:

1. Update `CHANGELOG.md` with changes
2. Tag a release:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```
3. GitHub Actions builds and publishes binaries to [Releases](https://github.com/miladbeigi/penhan/releases)

## License

MIT
