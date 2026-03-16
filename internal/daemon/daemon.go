package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/miekg/dns"
)

// DaemonConfig holds configuration for the daemon process.
type DaemonConfig struct {
	DNSAddr      string       // UDP address for DNS (e.g., "127.0.0.1:15353")
	ProxyAddr    string       // TCP address for HTTP proxy (e.g., ":80")
	RegistryPath string       // Path to registry.json
	Listener     net.Listener // Optional pre-bound listener (for launchd socket activation)
}

// Daemon coordinates the DNS server, HTTP proxy, and route watcher.
type Daemon struct {
	cfg    *DaemonConfig
	routes *RouteTable
	dns    *dns.Server
	proxy  *http.Server
}

// New creates a new Daemon instance.
func New(cfg *DaemonConfig) (*Daemon, error) {
	routes := &RouteTable{}
	proxyHandler := NewProxy(routes)
	routes.OnUpdate = proxyHandler.ClearCache

	dnsSrv := NewDNSServer(cfg.DNSAddr)
	httpSrv := &http.Server{
		Addr:    cfg.ProxyAddr,
		Handler: proxyHandler,
	}

	return &Daemon{
		cfg:    cfg,
		routes: routes,
		dns:    dnsSrv,
		proxy:  httpSrv,
	}, nil
}

// Run starts the daemon and blocks until the context is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	errCh := make(chan error, 3)

	// Start route watcher
	go func() {
		errCh <- WatchAndRebuild(ctx, d.cfg.RegistryPath, d.routes)
	}()

	// Start DNS server
	go func() {
		errCh <- d.dns.ListenAndServe()
	}()

	// Start HTTP proxy
	go func() {
		var err error
		if d.cfg.Listener != nil {
			err = d.proxy.Serve(d.cfg.Listener)
		} else {
			err = d.proxy.ListenAndServe()
		}
		if err == http.ErrServerClosed {
			err = nil
		}
		errCh <- err
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		d.dns.Shutdown()
		d.proxy.Close()
		return nil
	case err := <-errCh:
		d.dns.Shutdown()
		d.proxy.Close()
		return fmt.Errorf("daemon component failed: %w", err)
	}
}
