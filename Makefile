.DEFAULT_GOAL := help

.PHONY: ci fmt vet lint test test-integration test-e2e integration-up integration-down build tidy release help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X github.com/miladbeigi/penhan/internal/version.Version=$(VERSION) \
	-X github.com/miladbeigi/penhan/internal/version.Commit=$(COMMIT) \
	-X github.com/miladbeigi/penhan/internal/version.Date=$(DATE)

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

ci: fmt vet lint test build tidy ## Run all CI checks

fmt: ## Check code formatting
	@test -z "$$(gofmt -l .)" || (gofmt -d . && exit 1)

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run

test: ## Run unit tests
	go test ./...

integration-up: ## Start Vault container for integration tests
	cd integration && docker compose up -d
	@echo "Waiting for Vault to be ready..."
	@for i in $$(seq 1 30); do \
		if curl -sf http://127.0.0.1:18200/v1/sys/health > /dev/null 2>&1; then \
			echo "Vault is ready"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "Vault failed to start"; \
	exit 1

integration-down: ## Stop Vault container
	cd integration && docker compose down

test-integration: integration-up ## Run integration tests (requires Docker)
	go test -tags=integration -v -timeout 5m ./integration/...
	@make integration-down

test-e2e: ## Run e2e tests: real CLI against throwaway Vault containers (requires Docker)
	go test -tags=e2e -v -timeout 10m ./e2e/...

build: ## Build binary
	go build -ldflags "$(LDFLAGS)" -o penhan ./cmd/penhan

tidy: ## Check go mod tidy
	go mod tidy
	@git diff --exit-code go.mod
	@test ! -f go.sum || git diff --exit-code go.sum

release: ## Run GoReleaser to create a release
	goreleaser release --clean
