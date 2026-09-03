# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

The CLI is reduced to six commands: `add`, `check`, `push`, `encrypt`, `decrypt`, `version`.

### Added

- `penhan check`: hashes every local secret file and compares it with the backend, reporting each as `new`, `changed`, or `unchanged`. Reads only, never writes. Secrets that exist only in the backend are not reported.

### Changed

- `penhan add [name]` now creates a named safe (what `penhan safe add` used to do). A project is a directory of safes; there is no top-level `init`.
- `penhan push` runs the same comparison as `check`, pushes only new or changed secrets, and prints every secret it pushed or skipped. There is no plan, no confirmation prompt, and no `--force` flag.
- Conflict detection is gone along with the state file: the local file is the source of truth, and `push` overwrites edits made directly in the backend. Use `check` first to see them.
- The `.penhan/state.json` file is no longer created or read.

### Removed

- `penhan init`, `penhan safe add`, `penhan safe list`: replaced by `penhan add`
- `penhan plan`: replaced by `penhan check`
- `penhan pull`, `penhan list`: the encrypted `.enc` files in git are the copy you work from
- `penhan remove`: delete the secret file instead; removal from the backend is left to the backend's own tooling
- The old `penhan add <secret>` scaffolding command: secrets are plain files, so create them directly under the secrets directory

## [0.4.0] - 2026-09-02

### Added

- GitHub GPG provider: seal-only encryption using your GitHub public keys
- `penhan safe add [name]` and `penhan safe list`: create and inspect named subdirectory safes, each with its own `penhan.yaml`, keys, state, and Vault base path
- Interactive `init` backed by TUI prompts, with flags for non-interactive use
- `remove` accepts secret file paths (plaintext or `.enc`) and shows an interactive selection list when called with no argument
- E2E test suite running the real CLI against throwaway Vault containers (`make test-e2e`)

### Changed

- `add` no longer prompts for the secret value; it creates a `key: value` skeleton file to edit directly
- `pull` plans against the actual Vault state, so remote secrets are reported as "Add" instead of "Delete"

### Fixed

- Module path corrected to `github.com/miladbeigi/penhan`, so documented install commands resolve
- `remove` output shows the canonical Vault path with colored `local:`/`vault:` labels
- GoReleaser injects version metadata into the renamed module path, so released binaries report their real version

## [0.3.0] - 2026-08-27

### Changed

- A secret changed only on the remote is now a conflict: `push` no longer silently overwrites remote edits (use `--force` to override)
- `encrypt` removes the plaintext file after writing the `.enc`, mirroring `decrypt`
- `encrypt` and `decrypt` with no arguments default to the configured secrets directory
- `init` refuses to overwrite an existing `penhan.yaml`
- Runtime errors no longer print the cobra usage block

### Fixed

- Vault listing recurses into KV v2 folders: nested secrets (e.g. `apps/api-token`) are restored by `pull`, settle in `plan`, and participate in conflict detection
- `pull` skips unreadable entries (soft-deleted versions) instead of aborting the entire pull
- `list` no longer fails when `state.json` is missing and now shows encrypted-only secrets
- Secret files with nested maps or lists are rejected with a clear error instead of being silently corrupted (`map[a:1]`) in Vault
- `init` validates the Vault address up front instead of failing at first push with a cryptic URL parse error
- `plan` warns when the backend is unreachable instead of silently planning against an empty remote
- `add` warns when the secret value is empty

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
