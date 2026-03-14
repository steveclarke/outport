# Build the binary
build:
    go build -o outport .

# Run all tests
test:
    go test ./... -v

# Run tests (short output)
test-short:
    go test ./...

# Build and install to GOPATH/bin
install:
    go install .

# Run linter (requires: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint:
    golangci-lint run

# Clean build artifacts
clean:
    rm -f outport

# Build and run with args (e.g., just run up)
run *args:
    go run . {{args}}

# Show current version
version:
    go run . --version
