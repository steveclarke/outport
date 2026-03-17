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
