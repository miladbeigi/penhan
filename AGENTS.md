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

**Before committing:** Always run `go mod tidy && git diff --exit-code go.mod go.sum`. New direct imports must be moved from `// indirect` to direct in `go.mod` — CI's "Module tidy check" enforces this.

## Integration Tests

Integration tests require Docker and Vault:
```bash
make test-integration    # Starts Vault, runs tests, stops Vault
# or manually:
cd integration && docker compose up -d
go test -tags=integration ./integration/...
```

## Project Structure

```
cmd/penhan/          # CLI entrypoint (Cobra)
internal/
  backends/          # Vault provider (KV v2)
  commands/          # CLI commands (add, remove, push, pull, plan, etc.)
  config/            # YAML config parsing
  crypto/            # GPG/AES encryption providers
  prompt/            # TUI prompts (charmbracelet/huh)
  secrets/           # YAML/JSON parsing
  state/             # Sync state tracking (hash-based conflict detection)
```

## Key Concepts

- **State files:** `.penhan/state.json` tracks synced secrets via hash comparison
- **Conflict detection:** `GeneratePlan()` compares local vs remote hashes
- **Encrypted files:** `.enc` suffix; `.yaml`/`.yml`/`.json` for plaintext
- **Vault paths:** `secret/<base_path>/<relative_path>` (e.g., `secret/myapp/db/password`)

## Important Conventions

- New direct imports require `go mod tidy` before commit
- `golangci-lint v2.13.2` is the linter version (see CI workflow)
- Use `--force` flag for push/pull to override conflicts
- `safe add` creates a new safe; secrets live under `secrets/` directory

## Secrets File Format

Secrets are YAML/JSON files with simple key-value pairs:
```yaml
password: mysecret123
api_key: abc123
```

## Testing Single Components

```bash
go test ./internal/state/...     # Single package
go test -run TestPush ./...      # Single test
go test -tags=integration -run TestPull ./integration/...  # Single integration test
```