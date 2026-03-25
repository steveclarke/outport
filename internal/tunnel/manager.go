// Package tunnel provides a provider-agnostic abstraction for creating public
// tunnels to local services. A tunnel exposes a local port via a public URL so
// that a developer can share a running service with external collaborators or
// devices. The package is split into two layers: the Manager orchestrates
// tunnel lifecycles with all-or-nothing semantics, while Provider
// implementations (e.g., the cloudflare sub-package) handle the actual tunnel
// creation. State persistence is handled via a JSON file so that other
// commands can discover active tunnels.
package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager coordinates starting and stopping multiple tunnels concurrently.
// It holds a reference to a Provider (the tunnel backend) and enforces
// all-or-nothing semantics: if any single tunnel fails to start, all
// successfully started tunnels are torn down and an error is returned.
// The Manager is safe for concurrent use via an internal mutex.
type Manager struct {
	provider Provider
	timeout  time.Duration
	tunnels  []*Tunnel
	mu       sync.Mutex
}

// NewManager creates a Manager that uses the given provider and timeout.
// The timeout governs how long StartAll will wait for all tunnels to come up
// before cancelling. A typical timeout is 30 seconds, allowing time for the
// tunnel provider's external service to assign a public URL.
func NewManager(provider Provider, timeout time.Duration) *Manager {
	return &Manager{
		provider: provider,
		timeout:  timeout,
	}
}

// StartAll starts tunnels for all given services concurrently. The services map
// keys are service names (e.g., "web", "api") and values are the local ports
// to tunnel. Each tunnel is started in its own goroutine for parallelism.
//
// On success, all tunnels are stored in the Manager and returned. On failure,
// all-or-nothing semantics apply: any tunnels that did start successfully are
// stopped before the first error is returned. This prevents partially-exposed
// services that could confuse collaborators.
//
// The context is wrapped with the Manager's timeout. If any tunnel takes longer
// than the timeout to produce a URL, the context is cancelled and the
// remaining tunnels fail with a context error.
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
				results <- result{err: fmt.Errorf("%s: service %q: %w", m.provider.Name(), name, err)}
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

// StopAll stops all tunnels that were previously started by StartAll. It clears
// the Manager's internal tunnel list under the mutex, then stops each tunnel
// individually. Stop errors are silently ignored because tunnel processes may
// have already exited on their own. Safe to call multiple times.
func (m *Manager) StopAll() {
	m.mu.Lock()
	tunnels := m.tunnels
	m.tunnels = nil
	m.mu.Unlock()

	for _, tun := range tunnels {
		_ = tun.Stop()
	}
}
