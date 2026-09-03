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
go test -tags=e2e -run TestFullJourney ./e2e/...  # Single scenario
```

## Project Structure

```
cmd/penhan/          # CLI entrypoint (Cobra)
e2e/                 # E2E tests (real CLI against real Vault, build tag: e2e)
integration/         # Integration tests (real CLI against compose Vault, build tag: integration)
internal/
  backends/          # Backend providers: Vault KV v2, encrypted file
  commands/          # CLI commands: add, check, push, encrypt, decrypt, version
  config/            # YAML config parsing
  crypto/            # GPG / GitHub GPG / AES encryption providers
  prompt/            # TUI prompts (charmbracelet/huh)
  secrets/           # YAML/JSON parsing and path mapping
```

## Testing Single Components

```bash
go test ./internal/commands/...  # Single package
go test -run TestCheck ./...     # Single test
go test -tags=integration -run TestPush ./integration/...  # Single integration test
```
