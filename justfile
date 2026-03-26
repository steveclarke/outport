# Install dev tools (gotestsum, golangci-lint, goreleaser)
setup:
    go install gotest.tools/gotestsum@latest
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
    go install github.com/goreleaser/goreleaser/v2@latest
    go install github.com/securego/gosec/v2/cmd/gosec@v2.25.0
    go install golang.org/x/vuln/cmd/govulncheck@latest
    npm install

# Build the binary to dist/
build:
    @mkdir -p dist
    go build -ldflags "-X github.com/steveclarke/outport/cmd.version=dev-$(git rev-parse --short HEAD) -X github.com/steveclarke/outport/cmd.commit=$(git rev-parse --short HEAD) -X github.com/steveclarke/outport/cmd.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o dist/outport .

# Run all tests (colored output)
test:
    gotestsum --format testdox ./...

# Run tests (short output)
test-short:
    gotestsum --format dots ./...

# Run E2E tests with BATS (builds binary first)
test-e2e:
    just build
    bats e2e/

# Run all tests on Linux via Docker
test-linux:
    docker build -f docker/Dockerfile.test -t outport-test-linux .
    docker run --rm outport-test-linux

# Open shell in Linux dev container (starts it if not running)
dev-linux:
    docker compose up -d --build dev
    docker compose exec dev bash

# Stop Linux dev environment
dev-linux-down:
    docker compose down

# Install dev build to ~/.local/bin (overrides Homebrew)
install:
    @mkdir -p ~/.local/bin
    go build -ldflags "-X github.com/steveclarke/outport/cmd.version=dev-$(git rev-parse --short HEAD) -X github.com/steveclarke/outport/cmd.commit=$(git rev-parse --short HEAD) -X github.com/steveclarke/outport/cmd.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o ~/.local/bin/outport .
    @echo "Installed dev build to ~/.local/bin/outport"
    @echo "Run 'just uninstall' to switch back to Homebrew"

# Remove dev build (switch back to Homebrew)
uninstall:
    rm -f ~/.local/bin/outport
    rm -f $(go env GOPATH)/bin/outport
    @echo "Removed dev build. Using Homebrew version:"
    @outport --version 2>/dev/null || echo "  (not installed via Homebrew)"

# Show which outport binary is active
which:
    @which outport
    @outport --version

# Run linter
lint:
    golangci-lint run

# Run security scanner (source code)
gosec:
    gosec -exclude=G104,G204,G301,G304,G306 ./...

# Run vulnerability check (dependencies)
vulncheck:
    govulncheck ./...

# Clean build artifacts
clean:
    rm -rf dist

# Build and run with args (e.g., just run up)
run *args:
    go run . {{args}}

# Show current version
version:
    go run . --version

# Dry-run release (test GoReleaser config locally)
release-dry-run:
    goreleaser release --snapshot --clean

# Tag and push a release (e.g., just release v0.1.0)
release tag:
    git tag {{tag}}
    git push origin {{tag}}

# Start VitePress dev server (requires: npm install)
docs:
    npm run docs:dev

# Build VitePress site for production
docs-build:
    npm run docs:build

# Preview production build
docs-preview:
    npm run docs:preview
