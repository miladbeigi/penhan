# Agent Instructions for Penhan

## Running CI Checks Locally

```bash
make ci
```

Or run individually:
```bash
go build ./cmd/penhan       # Build
go vet ./...                 # Vet
go test ./...                # Unit tests only
make test-integration        # Integration tests (requires Docker)
```

**Before committing:** Always run all ci checks locally.

## Integration Tests

Integration tests require Docker and Vault:

```bash
make test-integration    # Starts Vault, runs tests, stops Vault

# or manually:
cd integration && docker compose up -d
go test -tags=integration ./integration/...
```

## E2E Tests

E2E tests run the real penhan binary against a real Vault server and a real
Kubernetes API server (k3s). Each Vault scenario starts its own throwaway
Vault container; the Kubernetes scenarios share one k3s container per test
(it takes ~20s to boot) and isolate themselves by namespace. Everything runs
via testcontainers-go on random ports and is destroyed afterwards — no
Makefile orchestration, no leftover state:

```bash
make test-e2e             # Requires Docker
go test -tags=e2e -run TestFullJourney ./e2e/...  # Single scenario
go test -tags=e2e -run TestKubernetesBackend ./e2e/...  # Kubernetes scenarios
```

## Project Structure

```
cmd/penhan/          # CLI entrypoint (Cobra)
docs/                # User documentation (linked from README.md); demo/ holds the VHS demo
e2e/                 # E2E tests (real CLI against real Vault and k3s, build tag: e2e)
integration/         # Integration tests (real CLI against compose Vault, build tag: integration)
internal/
  backends/          # Backend providers: Vault KV v2, Kubernetes Secrets, encrypted file
  commands/          # CLI commands: add, check, push, encrypt, decrypt, import, update, version
  config/            # YAML config parsing
  crypto/            # GPG / AES encryption providers
  prompt/            # TUI prompts (charmbracelet/huh)
  secrets/           # YAML/JSON parsing and path mapping
  update/            # Self-update from GitHub releases
.github/scripts/     # release-plan.sh: decides releases from CHANGELOG.md
```

## Documentation

The README is a short overview; details live in `docs/`. When a change affects users, update the matching page in `docs/` and add an entry under `## [Unreleased]` in `CHANGELOG.md`. Releases are cut by versioning that section (see CONTRIBUTING.md). If CLI output changes, re-record the demo with `make demo`.

## Testing Single Components

```bash
go test ./internal/commands/...  # Single package
go test -run TestCheck ./...     # Single test
go test -tags=integration -run TestPush ./integration/...  # Single integration test
```
