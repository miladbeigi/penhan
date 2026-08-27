.DEFAULT_GOAL := help

.PHONY: ci fmt vet lint test build tidy release help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X github.com/milad/penhan/internal/version.Version=$(VERSION) \
	-X github.com/milad/penhan/internal/version.Commit=$(COMMIT) \
	-X github.com/milad/penhan/internal/version.Date=$(DATE)

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

ci: fmt vet lint test build tidy ## Run all CI checks

fmt: ## Check code formatting

vet: ## Run go vet

lint: ## Run golangci-lint

test: ## Run tests

build: ## Build binary

tidy: ## Check go mod tidy

release: ## Run GoReleaser to create a release
