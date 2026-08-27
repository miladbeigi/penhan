# Penhan

[![CI](https://github.com/milad/penhan/actions/workflows/ci.yml/badge.svg)](https://github.com/milad/penhan/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/milad/penhan)](https://github.com/milad/penhan/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/milad/penhan)](https://goreportcard.com/report/github.com/milad/penhan)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Git-native secret management with encryption and Vault sync.**

Penhan manages secrets securely using Git as the source of truth. Secrets are stored encrypted in Git repositories and synced to HashiCorp Vault for application consumption.

## Features

- **Git-native** — secrets live in Git, versioned and auditable
- **Encrypted at rest** — GPG/PGP or AES-256-GCM encryption in the repository
- **Vault sync** — push secrets to HashiCorp Vault KV v2 for application use
- **Conflict safety** — Terraform-style plan/apply prevents accidental overwrites
- **Directory mapping** — local folder structure maps to Vault paths automatically
- **State tracking** — hash-based conflict detection across local and remote state

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

# Push secrets to Vault
penhan push

# Pull secrets from Vault
penhan pull
```

## Commands

| Command | Description |
|---------|-------------|
| `penhan init` | Initialize penhan in the current directory |
| `penhan add <name>` | Create a new secret |
| `penhan remove <name>` | Remove a secret from local and Vault |
| `penhan list` | List all secrets and their status |
| `penhan push` | Encrypt and sync local secrets to Vault |
| `penhan pull` | Fetch secrets from Vault and decrypt locally |
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

### Vault Path Mapping

Local paths map to Vault paths automatically:

| Local Path | Vault Path |
|------------|------------|
| `secrets/db/password.yaml` | `secret/data/db/password` |
| `secrets/api/key.yaml` | `secret/data/api/key` |

### Encryption

Penhan supports two encryption methods:

- **GPG/PGP** — uses your GPG keypair for encryption
- **AES-256-GCM** — symmetric encryption with a generated key

### State Management

Penhan tracks secret state using hashes:

- **new** — secret exists locally but hasn't been pushed
- **synced** — local and Vault are in sync
- **local_changed** — local version differs from last sync
- **remote_changed** — Vault version differs from last sync
- **conflict** — both sides changed since last sync

## Configuration

`penhan.yaml` in your project root:

```yaml
encryption:
  method: aes  # or "gpg"

backend:
  type: vault
  vault:
    addr: https://vault.example.com
    token_path: .penhan/vault-token
    mount_path: secret
    base_path: ""

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
