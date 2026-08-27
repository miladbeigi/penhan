# Penhan

[![CI](https://github.com/miladbeigi/penhan/actions/workflows/ci.yml/badge.svg)](https://github.com/miladbeigi/penhan/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/miladbeigi/penhan)](https://github.com/miladbeigi/penhan/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Git-native secret management with encryption and multi-backend sync.**

Penhan manages secrets securely using Git as the source of truth. Secrets are stored encrypted in Git repositories and synced to your preferred secret backend — HashiCorp Vault, AWS Secrets Manager, or other cloud secret services.

## Features

- **Git-native** — secrets live in Git, versioned and auditable
- **Encrypted at rest** — GPG/PGP or AES-256-GCM encryption in the repository
- **Multi-backend sync** — push secrets to Vault, AWS Secrets Manager, and more
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

Download a pre-built binary from the [latest release](https://github.com/milad/penhan/releases/latest):

```bash
# macOS (Apple Silicon)
curl -Lo penhan.tar.gz https://github.com/milad/penhan/releases/latest/download/penhan_*_darwin_arm64.tar.gz
tar xzf penhan.tar.gz
sudo mv penhan /usr/local/bin/

# Linux (amd64)
curl -Lo penhan.tar.gz https://github.com/milad/penhan/releases/latest/download/penhan_*_linux_amd64.tar.gz
tar xzf penhan.tar.gz
sudo mv penhan /usr/local/bin/
```

### Go Install

Requires Go 1.25+:

```bash
go install github.com/milad/penhan/cmd/penhan@latest
```

### Build from Source

```bash
git clone https://github.com/milad/penhan.git
cd penhan
make build
```

## Quick Start

```bash
# Initialize penhan in your project
penhan init

# Add a secret
penhan add db/password

# List secrets
penhan list

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
| `penhan add <name>` | Create a new secret |
| `penhan remove <name>` | Remove a secret from local and backend |
| `penhan list` | List all secrets and their status |
| `penhan push` | Encrypt and sync local secrets to backend |
| `penhan pull` | Fetch secrets from backend and decrypt locally |
| `penhan plan` | Show what push/pull would do (dry-run) |
| `penhan encrypt [file\|dir]` | Encrypt secret files in place |
| `penhan decrypt [file\|dir]` | Decrypt secret files in place |
| `penhan version` | Print version information |

## How It Works

### Directory Structure

```
my-project/
├── penhan.yaml              # Penhan configuration
├── secrets/                 # Secret files (encrypted in Git)
│   ├── db/
│   │   └── password.yaml
│   └── api/
│       └── key.yaml
└── .penhan/                 # Penhan state (gitignored)
    ├── state.json
    ├── keys/
    │   └── aes.key
    └── vault-token
```

### Path Mapping

Local paths map to backend paths automatically:

| Local Path | Vault Path | AWS Secrets Manager |
|------------|------------|---------------------|
| `secrets/db/password.yaml` | `secret/data/db/password` | `db/password` |
| `secrets/api/key.yaml` | `secret/data/api/key` | `api/key` |

### Encryption

Penhan supports two encryption methods:

- **GPG/PGP** — uses your GPG keypair for encryption
- **AES-256-GCM** — symmetric encryption with a generated key

### State Management

Penhan tracks secret state using hashes:

- **new** — secret exists locally but hasn't been pushed
- **synced** — local and remote are in sync
- **local_changed** — local version differs from last sync
- **remote_changed** — remote version differs from last sync
- **conflict** — both sides changed since last sync

## Configuration

`penhan.yaml` in your project root:

```yaml
encryption:
  method: aes  # or "gpg"

backend:
  type: vault  # or "aws" (planned)
  vault:
    addr: https://vault.example.com
    token_path: .penhan/vault-token
    mount_path: secret
    base_path: ""
  # aws:
  #   region: us-east-1
  #   prefix: myapp/

secrets:
  path: secrets/
  format: yaml
```

## Development

```bash
# Run tests
make test

# Run all CI checks
make ci

# Build binary
make build

# Run linter
make lint
```

## Release Process

Releases are automated via GitHub Actions and GoReleaser:

1. Update `CHANGELOG.md` with changes
2. Tag a release:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```
3. GitHub Actions builds and publishes binaries to [Releases](https://github.com/milad/penhan/releases)

## License

MIT
