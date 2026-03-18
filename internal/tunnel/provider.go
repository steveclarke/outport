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

// NewTunnel creates a Tunnel with the given parameters.
func NewTunnel(url string, port int, stop func() error) *Tunnel {
	return &Tunnel{
		URL:  url,
		Port: port,
		stop: stop,
	}
}
