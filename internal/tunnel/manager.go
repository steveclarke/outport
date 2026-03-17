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
