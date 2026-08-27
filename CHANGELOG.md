# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.2.0] - 2026-08-27

### Added

- Integration test suite running the real CLI against a Docker Compose Vault (`make test-integration`)
- `penhan remove <name> --force` to skip the confirmation prompt, matching `push`/`pull`
- `push` and `plan` now show which secrets are in conflict

### Fixed

- `init` did not write `encryption.<method>.key_path` to `penhan.yaml`, leaving later commands without a key
- AES provider regenerated a new key on every invocation instead of loading the existing one, making decryption across processes impossible
- `push` sent raw YAML to the Vault backend, which requires JSON; secrets are now parsed and pushed as canonical JSON
- `push` compared against an always-empty remote state, so conflicts were never detected; it now hashes local files and remote content against the last-synced state
- State was corrupted after a successful push (local hash overwritten with an empty remote hash)
- `pull` never found any secrets: Vault KV v2 listing used the `data/` path instead of `metadata/`
- `plan` did not account for local secret files that were never pushed
- `make integration-up` reported failure even when Vault started successfully

## [0.1.0] - 2026-08-27

### Added

- Initialize penhan with `penhan init`
- Add secrets with `penhan add <name>`
- Remove secrets with `penhan remove <name>`
- List secrets with `penhan list`
- Push secrets to backend with `penhan push`
- Pull secrets from backend with `penhan pull`
- Plan changes with `penhan plan` (dry-run)
- Encrypt files in place with `penhan encrypt`
- Decrypt files in place with `penhan decrypt`
- GPG/PGP encryption support via ProtonMail/go-crypto
- AES-256-GCM encryption support
- HashiCorp Vault KV v2 backend (first supported backend)
- Pluggable backend architecture for future providers
- Terraform-style plan/apply conflict detection
- Hash-based state management
- Directory hierarchy to backend path mapping
- Config file support (`penhan.yaml`)
- State file tracking (`.penhan/state.json`)
- GitHub Actions CI workflow (test, lint, vet, build, tidy)
- GitHub Actions Release workflow with GoReleaser
- GoReleaser config for cross-platform binary releases
- Version command with build metadata
- Professional README with badges, install instructions, and usage examples
- Makefile with build targets and version injection

### Fixed

- golangci-lint config updated for v2
- Unchecked error returns in GPG provider
