# Penhan

[![CI](https://github.com/miladbeigi/penhan/actions/workflows/ci.yml/badge.svg)](https://github.com/miladbeigi/penhan/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/miladbeigi/penhan)](https://github.com/miladbeigi/penhan/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Git-native secret management with encryption and multi-backend sync.**

Penhan manages secrets securely using Git as the source of truth. Secrets are stored encrypted in Git repositories and synced to your secret backend — HashiCorp Vault today, with more backends planned.

## Features

- **Git-native** — secrets live in Git, versioned and auditable
- **Encrypted at rest** — GPG/PGP, AES-256-GCM, or GitHub GPG keys
- **Multi-backend sync** — push secrets to Vault (more backends planned)
- **Named safes** — multiple isolated secret directories sharing one backend
- **Conflict safety** — Terraform-style plan/apply prevents accidental overwrites
- **Directory mapping** — local folder structure maps to backend paths automatically
- **State tracking** — hash-based conflict detection across local and remote state

## Backends

| Backend | Status |
|---------|--------|
| HashiCorp Vault KV v2 | Supported |
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
# Initialize penhan in your project (interactive)
penhan init

# Or non-interactively with flags
penhan init \
  --encryption=aes \
  --backend=vault \
  --vault-addr=https://vault.example.com \
  --vault-token-file=./vault-token

# Create a secret file: a flat YAML or JSON key-value map
mkdir -p secrets/db
echo "password: hunter2" > secrets/db/password.yaml

# Encrypt it in place for committing to Git
penhan encrypt

# See what would change
penhan plan

# Push secrets to backend
penhan push

# Pull secrets from backend
penhan pull
```

## Commands

| Command | Description |
|---------|-------------|
| `penhan init` | Initialize penhan in the current directory |
| `penhan list` | List all secrets and their status |
| `penhan push` | Encrypt and sync local secrets to backend (`--force` overrides conflicts) |
| `penhan pull` | Fetch secrets from backend and decrypt locally (`--force` overrides conflicts) |
| `penhan plan` | Show what push/pull would do (dry-run) |
| `penhan encrypt [file\|dir]` | Encrypt secret files in place (defaults to the secrets directory) |
| `penhan decrypt [file\|dir]` | Decrypt secret files in place (defaults to the secrets directory) |
| `penhan safe add [name]` | Create and initialize a new named safe |
| `penhan safe list` | List safes in the current directory |
| `penhan version` | Print version information |

## How It Works

### Directory Structure

```
my-project/
├── penhan.yaml                # Penhan configuration (committed)
├── secrets/                   # Secret files (the *.enc copies are committed)
│   ├── db/
│   │   └── password.yaml.enc  # Encrypted file in Git
│   └── api/
│       └── key.yaml.enc
└── .penhan/                   # Penhan state (gitignored)
    ├── state.json
    ├── keys/
    │   └── aes.key            # Key file named after the encryption method
    └── vault-token
```

You create the plaintext file yourself (or get it from `pull` + `decrypt`); `encrypt` replaces it with the `.enc` copy, and `push` syncs it to the backend.

### Safes

`penhan safe add <name>` creates a named safe — a subdirectory with its own `penhan.yaml`, `secrets/`, and `.penhan/` state:

```bash
penhan safe add myapp    # creates ./myapp with its own config and keys
penhan safe list         # lists safes found in the current directory
```

Each safe sets `backend.vault.base_path` to its name, so multiple safes can share one Vault backend without path collisions.

### Path Mapping

Local paths map to backend paths automatically:

| Local Path | Vault Path |
|------------|------------|
| `secrets/db/password.yaml` | `secret/data/db/password` |
| `secrets/api/key.yaml` | `secret/data/api/key` |

The Vault path is `{mount_path}/data/{base_path}/{secret path}`. `base_path` is empty for `penhan init` and set to the safe name for `penhan safe add`.

### Encryption

Penhan supports three encryption methods:

- **`gpg`** — uses your GPG keypair for encryption
- **`github-gpg`** — fetches your public key from `https://github.com/<username>.gpg` and encrypts with it; seal-only, so decrypting requires your private key on the local machine
- **`aes`** — symmetric AES-256-GCM encryption with a generated key stored under `.penhan/keys/`

Secret files are flat YAML or JSON key-value pairs (`.yaml`, `.yml`, or `.json`); nested maps or lists are rejected.

### State Management

Penhan tracks secret state using hashes:

- **new** — secret exists locally but hasn't been pushed
- **synced** — local and remote are in sync
- **local_changed** — local version differs from last sync
- **remote_changed** — remote version differs from last sync
- **conflict** — both sides changed since last sync (requires `--force` to override)

## Configuration

`penhan.yaml` in your project root (generated by `penhan init`):

```yaml
encryption:
  method: aes          # gpg, github-gpg, or aes
  aes:
    key_path: .penhan/keys/aes.key
  # gpg:
  #   key_path: .penhan/keys/gpg.key
  #   github_username: yourname  # required for github-gpg

backend:
  type: vault
  vault:
    addr: https://vault.example.com
    token_path: .penhan/vault-token
    mount_path: secret
    base_path: ""      # set to the safe name when initialized via `safe add`

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
