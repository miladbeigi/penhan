# Contributing

Thanks for your interest in penhan. Bug reports, fixes, and improvements are welcome. For larger changes, please open an issue first to agree on the approach.

## Development setup

You need Go (see `go.mod` for the version), Docker for the integration and end-to-end tests, and [golangci-lint](https://golangci-lint.run/). If you use [mise](https://mise.jdx.dev/), `mise install` sets up the pinned tool versions.

```bash
git clone https://github.com/miladbeigi/penhan.git
cd penhan
make build    # ./penhan
```

## Tests

| Command | What it runs |
|---|---|
| `make test` | Unit tests |
| `make test-integration` | The CLI against a Vault container started with Docker Compose |
| `make test-e2e` | The CLI against throwaway Vault and k3s containers (testcontainers-go) |
| `make ci` | Formatting, vet, lint, unit tests, build, and the `go mod tidy` check |

Run `make ci` before opening a pull request. CI runs all of the above, and all checks must pass before merging.

## Project layout

```
cmd/penhan/          CLI entry point
internal/backends/   Vault, Kubernetes, and file backends
internal/commands/   CLI commands
internal/config/     penhan.yaml parsing
internal/crypto/     AES and GPG encryption
internal/prompt/     Interactive prompts
internal/secrets/    Secret file parsing and path mapping
internal/update/     Self-update
integration/         Integration tests (build tag: integration)
e2e/                 End-to-end tests (build tag: e2e)
docs/                User documentation
```

## Documentation

User-facing changes should update the relevant page under [`docs/`](docs/) and add an entry under `## [Unreleased]` in [`CHANGELOG.md`](CHANGELOG.md).

The README demo is rendered from [`docs/demo/demo.tape`](docs/demo/demo.tape) with [VHS](https://github.com/charmbracelet/vhs). To re-record it after changing the CLI's output, install VHS and Docker, put the `penhan` binary you want to show on your `PATH`, and run:

```bash
make demo
```

It starts a throwaway Vault container, records, and removes the container.

## Releasing

Releases are published automatically from `CHANGELOG.md`:

1. Describe changes under `## [Unreleased]` as they land.
2. To release, open a pull request that renames `## [Unreleased]` to the new version and date, e.g. `## [0.7.0] - 2026-10-01`, with a fresh empty `## [Unreleased]` above it. Choose the version by [semantic versioning](https://semver.org/); while below 1.0, breaking changes bump the minor version.
3. Merge it. The [Release workflow](.github/workflows/release.yml) finds a changelog version without a tag, runs the full test suite, creates the tag, and publishes binaries with GoReleaser, using the changelog section as the release notes.

Merges that don't add a version section, including dependency updates, don't release anything. A version that isn't newer than the latest tag fails the workflow. Pushing a `vX.Y.Z` tag by hand also works, as long as `CHANGELOG.md` has a section for it. The logic lives in [`.github/scripts/release-plan.sh`](.github/scripts/release-plan.sh).
