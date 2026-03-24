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
