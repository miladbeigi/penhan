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

E2E tests run the real penhan binary against a real Vault server. Each
scenario starts its own throwaway Vault container (via testcontainers-go,
random port) and destroys it afterwards — no Makefile orchestration, no
leftover state:

```bash
make test-e2e             # Requires Docker
go test -tags=e2e -run TestSyncJourneyPushPull ./e2e/...  # Single scenario
```

## Project Structure

```
cmd/penhan/          # CLI entrypoint (Cobra)
e2e/                 # E2E tests (real CLI against real Vault, build tag: e2e)
internal/
  backends/          # Vault provider (KV v2)
  commands/          # CLI commands (remove, push, pull, plan, etc.)
  config/            # YAML config parsing
  crypto/            # GPG/AES encryption providers
  prompt/            # TUI prompts (charmbracelet/huh)
  secrets/           # YAML/JSON parsing
  state/             # Sync state tracking (hash-based conflict detection)
```

## Testing Single Components

```bash
go test ./internal/state/...     # Single package
go test -run TestPush ./...      # Single test
go test -tags=integration -run TestPull ./integration/...  # Single integration test
```
