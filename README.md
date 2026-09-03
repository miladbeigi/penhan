# Penhan

[![CI](https://github.com/miladbeigi/penhan/actions/workflows/ci.yml/badge.svg)](https://github.com/miladbeigi/penhan/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/miladbeigi/penhan)](https://github.com/miladbeigi/penhan/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Git-native secret management with encryption and backend sync.**

Penhan keeps secrets as encrypted files in Git and pushes them to a secret backend: HashiCorp Vault, or an encrypted directory on disk. Git is the source of truth; the backend is where applications read from.

## Features

- **Git-native** — secrets live in Git as encrypted files, versioned and auditable
- **Encrypted at rest** — GPG/PGP, AES-256-GCM, or GitHub GPG keys
- **Safes** — each safe is a directory with its own config, key, and backend base path
- **Hash-based sync** — `check` compares local files with the backend; `push` writes only what changed
- **Directory mapping** — the folder structure under `secrets/` maps to backend paths automatically
- **Six commands** — nothing to learn beyond add, check, push, encrypt, decrypt, version

## Backends

| Backend | Status |
|---------|--------|
| HashiCorp Vault KV v2 | Supported |
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
| `penhan decrypt [file\|dir]` | Decrypt secret files in place (defaults to the secrets directory) |
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

| Local Path | Vault Path |
|------------|------------|
| `secrets/db/password.yaml` | `secret/data/myapp/db/password` |
| `secrets/api/key.yaml` | `secret/data/myapp/api/key` |

The Vault path is `{mount_path}/data/{base_path}/{secret path}`, where `base_path` is the safe name.

### Encryption

Penhan supports three encryption methods:

- **`gpg`** — uses your GPG keypair for encryption
- **`github-gpg`** — fetches your public key from `https://github.com/<username>.gpg` and encrypts with it; seal-only, so decrypting requires your private key on the local machine
- **`aes`** — symmetric AES-256-GCM encryption with a generated key stored under `.penhan/keys/`

Secret files are flat YAML or JSON key-value pairs (`.yaml`, `.yml`, or `.json`); nested maps or lists are rejected. `check` and `push` read both plaintext and `.enc` files; when both exist for the same secret, the plaintext wins.

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
  method: aes          # gpg, github-gpg, or aes
  aes:
    key_path: .penhan/keys/aes.key
  # gpg:
  #   key_path: .penhan/keys/gpg.key
  #   github_username: yourname  # required for github-gpg

backend:
  type: vault          # vault or file
  vault:
    addr: https://vault.example.com
    token_path: .penhan/vault-token
    mount_path: secret
    base_path: myapp   # the safe name
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

# Run e2e tests: real CLI against throwaway Vault containers (requires Docker)
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
