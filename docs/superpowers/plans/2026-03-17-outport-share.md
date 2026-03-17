# outport share Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `outport share` command that tunnels HTTP services to public URLs via Cloudflare quick tunnels.

**Architecture:** Provider interface in `internal/tunnel/` with a Cloudflare implementation in `internal/tunnel/cloudflare/`. A tunnel manager coordinates multiple concurrent tunnels with all-or-nothing semantics. The `cmd/share.go` command wires it all together following existing Cobra patterns.

**Tech Stack:** Go, Cobra CLI, `os/exec` for subprocess management, `cloudflared` binary

---

## Chunk 1: Provider Interface & URL Parsing

### Task 1: Provider Interface Types

**Files:**
- Create: `internal/tunnel/provider.go`

- [ ] **Step 1: Create the provider interface and tunnel type**

```go
package tunnel

import "context"

// Provider is the interface that tunnel providers must implement.
// Designed for provider-agnosticism — if Cloudflare changes terms,
// swap the implementation without touching the manager or command.
type Provider interface {
	// Name returns the provider name (used in error messages).
	Name() string

	// CheckAvailable verifies the provider's dependencies are installed.
	CheckAvailable() error

	// Start creates a tunnel to the given local port.
	// It blocks until a public URL is obtained or ctx is cancelled.
	Start(ctx context.Context, port int) (*Tunnel, error)
}

// Tunnel represents an active tunnel connection.
type Tunnel struct {
	Service string
	URL     string
	Port    int
	stop    func() error
}

// Stop terminates the tunnel.
func (t *Tunnel) Stop() error {
	if t.stop != nil {
		return t.stop()
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/tunnel/`
Expected: Success, no errors

- [ ] **Step 3: Commit**

```bash
git add internal/tunnel/provider.go
git commit -m "feat(tunnel): add Provider interface and Tunnel type"
```

### Task 2: Cloudflare URL Parsing

The URL parser is extracted as a standalone function so it can be tested without running cloudflared.

**Files:**
- Create: `internal/tunnel/cloudflare/parse.go`
- Create: `internal/tunnel/cloudflare/parse_test.go`

- [ ] **Step 1: Write the failing test for URL parsing**

```go
package cloudflare

import "testing"

func TestParseURL(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name: "real cloudflared output",
			lines: []string{
				"2026-03-17T14:15:33Z INF Thank you for trying Cloudflare Tunnel.",
				"2026-03-17T14:15:33Z INF Requesting new quick Tunnel on trycloudflare.com...",
				"2026-03-17T14:15:40Z INF +--------------------------------------------------------------------------------------------+",
				"2026-03-17T14:15:40Z INF |  Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):  |",
				"2026-03-17T14:15:40Z INF |  https://soft-property-mas-trees.trycloudflare.com                                         |",
				"2026-03-17T14:15:40Z INF +--------------------------------------------------------------------------------------------+",
				"2026-03-17T14:15:40Z INF Cannot determine default configuration path.",
			},
			want: "https://soft-property-mas-trees.trycloudflare.com",
		},
		{
			name:  "no url in output",
			lines: []string{"some random log line", "another line"},
			want:  "",
		},
		{
			name: "url with numbers in subdomain",
			lines: []string{
				"2026-03-17T14:15:40Z INF |  https://abc-123-def-456.trycloudflare.com  |",
			},
			want: "https://abc-123-def-456.trycloudflare.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseURL(tt.lines)
			if got != tt.want {
				t.Errorf("parseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tunnel/cloudflare/ -run TestParseURL -v`
Expected: FAIL — `parseURL` not defined

- [ ] **Step 3: Implement the URL parser**

```go
package cloudflare

import "regexp"

// tunnelURLRe matches Cloudflare quick tunnel URLs in log output.
var tunnelURLRe = regexp.MustCompile(`https://[-a-z0-9]+\.trycloudflare\.com`)

// parseURL scans lines of cloudflared output for a tunnel URL.
// Returns the URL or empty string if not found.
func parseURL(lines []string) string {
	for _, line := range lines {
		if m := tunnelURLRe.FindString(line); m != "" {
			return m
		}
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tunnel/cloudflare/ -run TestParseURL -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tunnel/cloudflare/parse.go internal/tunnel/cloudflare/parse_test.go
git commit -m "feat(tunnel): add cloudflare URL parser with tests"
```

### Task 3: Cloudflare Provider Implementation

**Files:**
- Create: `internal/tunnel/cloudflare/cloudflare.go`
- Create: `internal/tunnel/cloudflare/cloudflare_test.go`

- [ ] **Step 1: Write the failing test for CheckAvailable**

```go
package cloudflare

import (
	"os"
	"testing"
)

func TestCheckAvailable_NotInstalled(t *testing.T) {
	// Use an empty PATH so cloudflared can't be found
	t.Setenv("PATH", t.TempDir())

	p := New()
	err := p.CheckAvailable()
	if err == nil {
		t.Fatal("expected error when cloudflared not in PATH")
	}
	if got := err.Error(); got != "cloudflared not found. Install with: brew install cloudflared" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestCheckAvailable_Installed(t *testing.T) {
	// Create a fake cloudflared binary in a temp dir
	dir := t.TempDir()
	fake := dir + "/cloudflared"
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	p := New()
	if err := p.CheckAvailable(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestName(t *testing.T) {
	p := New()
	if got := p.Name(); got != "cloudflare" {
		t.Errorf("Name() = %q, want %q", got, "cloudflare")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tunnel/cloudflare/ -run 'TestCheckAvailable|TestName' -v`
Expected: FAIL — `New` not defined

- [ ] **Step 3: Implement the Cloudflare provider**

```go
package cloudflare

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"

	"github.com/outport-app/outport/internal/tunnel"
)

// Provider implements tunnel.Provider using Cloudflare quick tunnels.
type Provider struct{}

// New creates a new Cloudflare tunnel provider.
func New() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string {
	return "cloudflare"
}

func (p *Provider) CheckAvailable() error {
	_, err := exec.LookPath("cloudflared")
	if err != nil {
		return fmt.Errorf("cloudflared not found. Install with: brew install cloudflared")
	}
	return nil
}

func (p *Provider) Start(ctx context.Context, port int) (*tunnel.Tunnel, error) {
	cmd := exec.CommandContext(ctx, "cloudflared", "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port))
	cmd.Stdout = nil // cloudflared writes nothing to stdout

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting cloudflared: %w", err)
	}

	urlCh := make(chan string, 1)
	errCh := make(chan error, 1)
	var lastLines []string

	// Scan stderr for the tunnel URL
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Keep last 5 lines for error diagnostics
			if len(lastLines) >= 5 {
				lastLines = lastLines[1:]
			}
			lastLines = append(lastLines, line)

			if url := parseURL([]string{line}); url != "" {
				urlCh <- url
				// Keep draining stderr so cloudflared doesn't block on writes
				io.Copy(io.Discard, stderr)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("reading cloudflared output: %w", err)
		} else {
			errCh <- fmt.Errorf("cloudflared exited without producing a tunnel URL")
		}
	}()

	select {
	case url := <-urlCh:
		return tunnel.NewTunnel(url, port, stopFunc(cmd)), nil
	case err := <-errCh:
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, err
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, ctx.Err()
	}
}

// stopFunc returns a function that gracefully stops the cloudflared process.
func stopFunc(cmd *exec.Cmd) func() error {
	return func() error {
		if cmd.Process == nil {
			return nil
		}
		// Send SIGTERM for graceful shutdown
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			// Process already exited
			if cmd.ProcessState != nil {
				return nil
			}
			return err
		}
		// Wait up to 3 seconds for graceful exit
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
			return nil
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			return nil
		}
	}
}
```

- [ ] **Step 4: Add NewTunnel constructor to provider.go**

Update `internal/tunnel/provider.go` to add the constructor:

```go
// NewTunnel creates a Tunnel with the given parameters.
func NewTunnel(url string, port int, stop func() error) *Tunnel {
	return &Tunnel{
		URL:  url,
		Port: port,
		stop: stop,
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tunnel/cloudflare/ -run 'TestCheckAvailable|TestName' -v`
Expected: PASS

- [ ] **Step 6: Run lint**

Run: `just lint`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add internal/tunnel/provider.go internal/tunnel/cloudflare/cloudflare.go internal/tunnel/cloudflare/cloudflare_test.go
git commit -m "feat(tunnel): add Cloudflare provider with CheckAvailable and Start"
```

## Chunk 2: Tunnel Manager

### Task 4: Tunnel Manager

**Files:**
- Create: `internal/tunnel/manager.go`
- Create: `internal/tunnel/manager_test.go`

- [ ] **Step 1: Write the failing tests for the manager**

```go
package tunnel

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// mockProvider implements Provider for testing.
type mockProvider struct {
	name   string
	urls   map[int]string // port → URL
	delay  time.Duration
	failOn int // port that should fail
}

func (m *mockProvider) Name() string            { return m.name }
func (m *mockProvider) CheckAvailable() error   { return nil }
func (m *mockProvider) Start(ctx context.Context, port int) (*Tunnel, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if port == m.failOn {
		return nil, fmt.Errorf("tunnel failed for port %d", port)
	}
	url, ok := m.urls[port]
	if !ok {
		return nil, fmt.Errorf("unexpected port %d", port)
	}
	return NewTunnel(url, port, func() error { return nil }), nil
}

// trackingMockProvider wraps mockProvider to track Stop() calls.
type trackingMockProvider struct {
	mockProvider
	onStop func()
}

func (m *trackingMockProvider) Start(ctx context.Context, port int) (*Tunnel, error) {
	tun, err := m.mockProvider.Start(ctx, port)
	if err != nil {
		return nil, err
	}
	// Wrap the stop function to track calls
	return NewTunnel(tun.URL, tun.Port, func() error { m.onStop(); return nil }), nil
}

func TestManager_StartAll_Success(t *testing.T) {
	p := &mockProvider{
		name: "mock",
		urls: map[int]string{
			3000: "https://aaa.example.com",
			5173: "https://bbb.example.com",
		},
	}
	m := NewManager(p, 5*time.Second)

	services := map[string]int{"web": 3000, "vite": 5173}
	tunnels, err := m.StartAll(context.Background(), services)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer m.StopAll()

	if len(tunnels) != 2 {
		t.Fatalf("got %d tunnels, want 2", len(tunnels))
	}

	byService := make(map[string]*Tunnel)
	for _, tun := range tunnels {
		byService[tun.Service] = tun
	}
	if byService["web"].URL != "https://aaa.example.com" {
		t.Errorf("web URL = %q, want %q", byService["web"].URL, "https://aaa.example.com")
	}
	if byService["vite"].URL != "https://bbb.example.com" {
		t.Errorf("vite URL = %q, want %q", byService["vite"].URL, "https://bbb.example.com")
	}
}

func TestManager_StartAll_OneFails(t *testing.T) {
	stopCount := atomic.Int32{}
	p := &trackingMockProvider{
		mockProvider: mockProvider{
			name:   "mock",
			urls:   map[int]string{3000: "https://aaa.example.com", 5173: "https://bbb.example.com"},
			failOn: 5173,
		},
		onStop: func() { stopCount.Add(1) },
	}

	m := NewManager(p, 5*time.Second)

	services := map[string]int{"web": 3000, "vite": 5173}
	_, err := m.StartAll(context.Background(), services)
	if err == nil {
		t.Fatal("expected error when one tunnel fails")
	}

	// Verify the successful tunnel was cleaned up (all-or-nothing)
	if got := stopCount.Load(); got != 1 {
		t.Errorf("Stop() called %d times, want 1 (all-or-nothing cleanup)", got)
	}
}

func TestManager_StartAll_Timeout(t *testing.T) {
	p := &mockProvider{
		name:  "mock",
		urls:  map[int]string{3000: "https://aaa.example.com"},
		delay: 10 * time.Second, // longer than timeout
	}
	m := NewManager(p, 100*time.Millisecond)

	services := map[string]int{"web": 3000}
	_, err := m.StartAll(context.Background(), services)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestManager_StartAll_EmptyServices(t *testing.T) {
	p := &mockProvider{name: "mock"}
	m := NewManager(p, 5*time.Second)

	tunnels, err := m.StartAll(context.Background(), map[string]int{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tunnels) != 0 {
		t.Errorf("got %d tunnels, want 0", len(tunnels))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tunnel/ -run TestManager -v`
Expected: FAIL — `NewManager` not defined

- [ ] **Step 3: Implement the tunnel manager**

```go
package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager coordinates starting and stopping multiple tunnels.
type Manager struct {
	provider Provider
	timeout  time.Duration
	tunnels  []*Tunnel
	mu       sync.Mutex
}

// NewManager creates a Manager that uses the given provider and timeout.
func NewManager(provider Provider, timeout time.Duration) *Manager {
	return &Manager{
		provider: provider,
		timeout:  timeout,
	}
}

// StartAll starts tunnels for all given services concurrently.
// Returns all tunnels on success. If any tunnel fails, stops all
// successful tunnels and returns the error (all-or-nothing).
func (m *Manager) StartAll(ctx context.Context, services map[string]int) ([]*Tunnel, error) {
	if len(services) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	type result struct {
		tunnel *Tunnel
		err    error
	}

	results := make(chan result, len(services))

	for name, port := range services {
		go func(name string, port int) {
			tun, err := m.provider.Start(ctx, port)
			if err != nil {
				results <- result{err: fmt.Errorf("service %q: %w", name, err)}
				return
			}
			tun.Service = name
			results <- result{tunnel: tun}
		}(name, port)
	}

	var tunnels []*Tunnel
	var firstErr error

	for range services {
		r := <-results
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		tunnels = append(tunnels, r.tunnel)
	}

	if firstErr != nil {
		// All-or-nothing: stop the ones that succeeded
		for _, tun := range tunnels {
			_ = tun.Stop()
		}
		return nil, firstErr
	}

	m.mu.Lock()
	m.tunnels = tunnels
	m.mu.Unlock()

	return tunnels, nil
}

// StopAll stops all running tunnels.
func (m *Manager) StopAll() {
	m.mu.Lock()
	tunnels := m.tunnels
	m.tunnels = nil
	m.mu.Unlock()

	for _, tun := range tunnels {
		_ = tun.Stop()
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tunnel/ -run TestManager -v`
Expected: PASS

- [ ] **Step 5: Run lint**

Run: `just lint`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add internal/tunnel/manager.go internal/tunnel/manager_test.go
git commit -m "feat(tunnel): add Manager for concurrent tunnel orchestration"
```

## Chunk 3: Share Command

### Task 5: Share Command — Core Logic

**Files:**
- Create: `cmd/share.go`

- [ ] **Step 1: Implement the share command**

```go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/tunnel"
	"github.com/outport-app/outport/internal/tunnel/cloudflare"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var shareCmd = &cobra.Command{
	Use:   "share [service...]",
	Short: "Tunnel HTTP services to public URLs",
	Long:  "Creates public tunnel URLs for HTTP services using Cloudflare quick tunnels. Shares all HTTP services by default, or specify service names to share specific ones.",
	Args:  cobra.ArbitraryArgs,
	RunE:  runShare,
}

func init() {
	rootCmd.AddCommand(shareCmd)
}

func runShare(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		return fmt.Errorf("No ports allocated. Run 'outport apply' first.")
	}

	services, err := resolveShareServices(ctx, args)
	if err != nil {
		return err
	}

	provider := cloudflare.New()
	if err := provider.CheckAvailable(); err != nil {
		return err
	}

	mgr := tunnel.NewManager(provider, 15*time.Second)

	// Build service→port map
	svcPorts := make(map[string]int)
	for _, name := range services {
		svcPorts[name] = alloc.Ports[name]
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	tunnels, err := mgr.StartAll(sigCtx, svcPorts)
	if err != nil {
		return fmt.Errorf("starting tunnels: %w", err)
	}
	defer mgr.StopAll()

	if jsonFlag {
		return printShareJSON(cmd, tunnels)
	}
	printShareStyled(cmd, tunnels)

	// Block until signal
	<-sigCtx.Done()
	return nil
}

// resolveShareServices returns the sorted list of service names to share.
func resolveShareServices(ctx *projectContext, args []string) ([]string, error) {
	if len(args) > 0 {
		// Validate named services
		for _, name := range args {
			svc, ok := ctx.Cfg.Services[name]
			if !ok {
				return nil, FlagErrorf("unknown service %q", name)
			}
			if svc.Protocol != "http" && svc.Protocol != "https" {
				return nil, fmt.Errorf("service %q has no protocol and cannot be shared", name)
			}
		}
		sort.Strings(args)
		return args, nil
	}

	// Default: all HTTP services
	var services []string
	for name, svc := range ctx.Cfg.Services {
		if svc.Protocol == "http" || svc.Protocol == "https" {
			services = append(services, name)
		}
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no shareable services found. Add 'protocol: http' to a service in .outport.yml")
	}
	sort.Strings(services)
	return services, nil
}
```


- [ ] **Step 2: Add the output functions**

Add to the same `cmd/share.go` file:

```go
// JSON output types

type tunnelJSON struct {
	Service string `json:"service"`
	URL     string `json:"url"`
	Port    int    `json:"port"`
}

type shareJSON struct {
	Tunnels []tunnelJSON `json:"tunnels"`
}

func printShareJSON(cmd *cobra.Command, tunnels []*tunnel.Tunnel) error {
	out := shareJSON{}
	for _, tun := range tunnels {
		out.Tunnels = append(out.Tunnels, tunnelJSON{
			Service: tun.Service,
			URL:     tun.URL,
			Port:    tun.Port,
		})
	}
	// Sort for deterministic output
	sort.Slice(out.Tunnels, func(i, j int) bool {
		return out.Tunnels[i].Service < out.Tunnels[j].Service
	})
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printShareStyled(cmd *cobra.Command, tunnels []*tunnel.Tunnel) {
	w := cmd.OutOrStdout()

	// Sort tunnels by service name
	sort.Slice(tunnels, func(i, j int) bool {
		return tunnels[i].Service < tunnels[j].Service
	})

	lipgloss.Fprintln(w, fmt.Sprintf("Sharing %d %s:",
		len(tunnels), pluralize(len(tunnels), "service", "services")))
	lipgloss.Fprintln(w)

	for _, tun := range tunnels {
		line := fmt.Sprintf("  %s  %s %s localhost:%d",
			ui.ServiceStyle.Render(fmt.Sprintf("%-16s", tun.Service)),
			ui.UrlStyle.Render(tun.URL),
			ui.Arrow,
			tun.Port,
		)
		lipgloss.Fprintln(w, line)
	}

	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.DimStyle.Render("Press Ctrl+C to stop sharing."))
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/...`

If there's a build issue from `cmd/` not being a `main` package, use:

Run: `go build ./...`
Expected: Success

- [ ] **Step 4: Run lint**

Run: `just lint`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add cmd/share.go
git commit -m "feat: add outport share command"
```

### Task 6: Share Command Tests

**Files:**
- Modify: `cmd/cmd_test.go`

- [ ] **Step 1: Add a test config with HTTP services**

Add near the top of `cmd/cmd_test.go`, after the existing `testConfig`:

```go
const testConfigWithHTTP = `name: testapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
    protocol: http
    hostname: testapp.test
  vite:
    preferred_port: 5173
    env_var: VITE_PORT
    protocol: http
    hostname: testapp-vite.test
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT
`
```

- [ ] **Step 2: Write tests for error cases**

Add to `cmd/cmd_test.go`:

```go
// --- share ---

func TestShare_NoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	jsonFlag = false
	useHTTPS = false

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"share"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no .outport.yml exists")
	}
}

func TestShare_NoAllocation(t *testing.T) {
	setupProject(t, testConfigWithHTTP)

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"share"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no ports allocated")
	}
	want := "No ports allocated"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want containing %q", got, want)
	}
}

func TestShare_UnknownService(t *testing.T) {
	setupProject(t, testConfigWithHTTP)
	executeCmd(t, "apply") // allocate ports first

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"share", "nonexistent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
	if !IsFlagError(err) {
		t.Errorf("expected FlagError, got %T", err)
	}
}

func TestShare_ServiceWithoutProtocol(t *testing.T) {
	setupProject(t, testConfigWithHTTP)
	executeCmd(t, "apply")

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"share", "postgres"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for service without protocol")
	}
	want := "has no protocol"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want containing %q", got, want)
	}
}

func TestShare_NoHTTPServices(t *testing.T) {
	setupProject(t, testConfig) // testConfig has no protocol on any service
	executeCmd(t, "apply")

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"share"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no HTTP services")
	}
	want := "no shareable services"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want containing %q", got, want)
	}
}

func TestShare_CloudflaredNotInstalled(t *testing.T) {
	setupProject(t, testConfigWithHTTP)
	executeCmd(t, "apply")

	// Set PATH to empty dir so cloudflared can't be found
	t.Setenv("PATH", t.TempDir())

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"share"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when cloudflared not installed")
	}
	want := "cloudflared not found"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want containing %q", got, want)
	}
}

// Note: tests use strings.Contains from stdlib — add "strings" to the import block in cmd_test.go
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `go test ./cmd/ -run 'TestShare_' -v`
Expected: PASS for all error case tests

- [ ] **Step 4: Run full test suite**

Run: `just test`
Expected: All tests pass (existing + new)

- [ ] **Step 5: Run lint**

Run: `just lint`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add cmd/cmd_test.go
git commit -m "test: add share command error case tests"
```

## Chunk 4: Final Verification & Docs

### Task 8: Full Test Suite & Lint

- [ ] **Step 1: Run the full test suite**

Run: `just test`
Expected: All tests pass

- [ ] **Step 2: Run lint**

Run: `just lint`
Expected: No errors

- [ ] **Step 3: Manual smoke test**

Run from a project with `.outport.yml` and allocated ports:

```bash
just run share
```

Expected: Tunnels start, URLs displayed, Ctrl+C exits cleanly.

Also test:
```bash
just run share --json
just run share web
just run share nonexistent  # should error
```

### Task 9: Update Documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`
- Modify: `docs/reference/commands.md`

- [ ] **Step 1: Add share command to CLAUDE.md CLI commands list**

In the `### CLI commands` section, add after the `open` entry:

```
- **share** — Tunnel HTTP services to public URLs via Cloudflare quick tunnels. Shares all HTTP services by default, or specify service names. Requires `cloudflared` binary. Blocks until Ctrl+C.
```

- [ ] **Step 2: Add tunnel package to CLAUDE.md core packages list**

In the `### Core packages` section, add:

```
- **tunnel** — Tunnel provider abstraction and concurrent manager. Provider interface allows swapping tunnel backends (Cloudflare, etc.) without changing command code. Manager starts/stops multiple tunnels with all-or-nothing semantics and configurable timeout.
- **tunnel/cloudflare** — Cloudflare quick tunnel provider. Shells out to `cloudflared tunnel --url`, parses tunnel URL from stderr output.
```

- [ ] **Step 3: Add share to README.md commands list**

In the `## Commands` section, add after the `outport open` line:

```
outport share              Tunnel HTTP services to public URLs
outport share web          Tunnel a specific service
```

- [ ] **Step 4: Add share to docs site commands page**

In `docs/reference/commands.md`, add a new section after `### outport open` (in the Navigation section):

```markdown
### `outport share`

Tunnel HTTP services to public URLs via Cloudflare quick tunnels.

\`\`\`bash
outport share              # tunnel all HTTP services
outport share web          # tunnel a specific service
outport share web vite     # tunnel specific services
\`\`\`

Creates temporary public URLs for services with `protocol: http` or `protocol: https`. Requires `cloudflared` (`brew install cloudflared`). The command blocks until you press Ctrl+C.

| Flag | Description |
|------|-------------|
| `--json` | Output tunnel URLs as JSON |
```

- [ ] **Step 5: Verify docs site builds**

Run: `npm run docs:build`
Expected: Build succeeds

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md README.md docs/reference/commands.md
git commit -m "docs: add share command to CLAUDE.md, README, and docs site"
```

### Task 10: Update Issue

- [ ] **Step 1: Update issue #16 with implementation status**

```bash
gh issue comment 16 --body "Implemented in $(git rev-parse --short HEAD). Core `outport share` command working with Cloudflare quick tunnels, provider abstraction for future flexibility."
```
