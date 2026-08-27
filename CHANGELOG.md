# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- GitHub Actions CI workflow (test, lint, vet, build, tidy)
- GitHub Actions Release workflow with GoReleaser
- GoReleaser config for cross-platform binary releases
- Version command with build metadata
- Professional README with badges, install instructions, and usage examples
- CHANGELOG.md following Keep a Changelog format
- Makefile release target

### Fixed

- golangci-lint config updated for v2
- Unchecked error returns in GPG provider

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
