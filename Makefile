.PHONY: ci fmt vet lint test build tidy release

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X github.com/milad/penhan/internal/version.Version=$(VERSION) \
	-X github.com/milad/penhan/internal/version.Commit=$(COMMIT) \
	-X github.com/milad/penhan/internal/version.Date=$(DATE)

ci: fmt vet lint test build tidy

fmt:
	@test -z "$$(gofmt -l .)" || (gofmt -d . && exit 1)

vet:
	go vet ./...

lint:
	golangci-lint run

test:
	go test ./...

build:
	go build -ldflags "$(LDFLAGS)" -o penhan ./cmd/penhan

tidy:
	go mod tidy
	@git diff --exit-code go.mod
	@test ! -f go.sum || git diff --exit-code go.sum

release:
	goreleaser release --clean
