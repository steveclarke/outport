# Install dev tools (gotestsum, golangci-lint, goreleaser)
setup:
    go install gotest.tools/gotestsum@latest
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install github.com/goreleaser/goreleaser/v2@latest
    npm install

# Build the binary to dist/
build:
    @mkdir -p dist
    go build -o dist/outport .

# Run all tests (colored output)
test:
    gotestsum --format testdox ./...

# Run tests (short output)
test-short:
    gotestsum --format dots ./...

# Install dev build to ~/.local/bin (overrides Homebrew)
install:
    @mkdir -p ~/.local/bin
    go build -ldflags "-X github.com/outport-app/outport/cmd.version=dev-$(git rev-parse --short HEAD)" -o ~/.local/bin/outport .
    @echo "Installed dev build to ~/.local/bin/outport"
    @echo "Run 'just uninstall' to switch back to Homebrew"

# Remove dev build (switch back to Homebrew)
uninstall:
    rm -f ~/.local/bin/outport
    @echo "Removed dev build. Using Homebrew version:"
    @outport --version 2>/dev/null || echo "  (not installed via Homebrew)"

# Show which outport binary is active
which:
    @which outport
    @outport --version

# Run linter (requires: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint:
    golangci-lint run

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
