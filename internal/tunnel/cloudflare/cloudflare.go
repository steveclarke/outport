// Package cloudflare implements the tunnel.Provider interface using Cloudflare's
// free "quick tunnel" feature. Quick tunnels require no account or configuration
// — the cloudflared CLI is invoked with a local port and it assigns a random
// public *.trycloudflare.com URL. This is used by the "outport share" command
// to let developers share local services with external collaborators.
//
// The implementation shells out to the cloudflared binary, parses the assigned
// URL from its stderr output, and manages the process lifecycle with graceful
// SIGTERM shutdown.
package cloudflare

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/steveclarke/outport/internal/tunnel"
)

// Provider implements the tunnel.Provider interface using Cloudflare quick
// tunnels. It has no configuration or state — each call to Start spawns a new
// cloudflared process. The cloudflared CLI must be installed on the system
// (typically via "brew install cloudflare/cloudflare/cloudflared").
type Provider struct{}

// New creates a new Cloudflare tunnel provider. The provider is stateless, so
// a single instance can be reused for multiple tunnel creations.
func New() *Provider {
	return &Provider{}
}

// Name returns "cloudflare", used in error messages and logging to identify
// which tunnel provider is in use.
func (p *Provider) Name() string {
	return "cloudflare"
}

// CheckAvailable verifies that the cloudflared binary is installed and
// available on the system PATH. Returns a user-friendly error with install
// instructions if the binary is not found.
func (p *Provider) CheckAvailable() error {
	_, err := exec.LookPath("cloudflared")
	if err != nil {
		return fmt.Errorf("cloudflared not found. Install with: brew install cloudflared")
	}
	return nil
}

// Start creates a new Cloudflare quick tunnel pointing to the given local port.
// It spawns a cloudflared process, scans its stderr for the assigned public URL
// (a *.trycloudflare.com address), and returns a Tunnel with that URL and a
// stop function for cleanup.
//
// The context controls the timeout for URL discovery. If the context is
// cancelled before a URL appears in the output, the cloudflared process is
// killed and a context error is returned.
//
// Note: exec.Command is used instead of exec.CommandContext intentionally. The
// context is only used for the startup timeout — the cloudflared process
// lifecycle is managed by the returned Tunnel's Stop function, not by context
// cancellation.
func (p *Provider) Start(ctx context.Context, port int) (*tunnel.Tunnel, error) {
	// Use exec.Command (not CommandContext) so the process isn't killed when
	// the manager's timeout context expires. Lifecycle is managed by stopFunc.
	cmd := exec.Command("cloudflared", "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port))
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

	// Scan stderr for the tunnel URL
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if url := parseURL(line); url != "" {
				urlCh <- url
				// Keep draining stderr so cloudflared doesn't block on writes
				io.Copy(io.Discard, stderr) //nolint:errcheck
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
// Safe to call multiple times via sync.Once.
func stopFunc(cmd *exec.Cmd) func() error {
	var once sync.Once
	var stopErr error
	return func() error {
		once.Do(func() {
			if cmd.Process == nil {
				return
			}
			// Send SIGTERM for graceful shutdown. Ignore error — if the
			// process is already dead, Wait() will return immediately.
			_ = cmd.Process.Signal(syscall.SIGTERM)

			// Wait up to 3 seconds for graceful exit
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()

			timer := time.NewTimer(3 * time.Second)
			defer timer.Stop()

			select {
			case <-done:
			case <-timer.C:
				_ = cmd.Process.Kill()
				<-done
			}
		})
		return stopErr
	}
}
