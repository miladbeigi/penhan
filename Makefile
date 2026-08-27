.PHONY: ci fmt vet lint test build tidy

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
	go build -o penhan ./cmd/penhan

tidy:
	go mod tidy
	@git diff --exit-code go.mod
	@test ! -f go.sum || git diff --exit-code go.sum
