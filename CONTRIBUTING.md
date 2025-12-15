# Contributing

## Setup

Enable the pre-commit hook:

```bash
git config core.hooksPath .githooks
```

Install golangci-lint:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Pre-commit checks

The pre-commit hook runs automatically on staged `.go` files:

1. `gofmt -s` - Format check
2. `golangci-lint` - Linting
3. `go test` - Tests
4. `go build` - Build verification

## Manual checks

```bash
# Format
gofmt -s -w .

# Lint
golangci-lint run ./...

# Test
make test

# Build
make
```
