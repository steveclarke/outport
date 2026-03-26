// Package daemon implements the long-running background process that powers
// Outport's local development proxy. The daemon runs three core servers:
// a DNS server that resolves all *.test hostnames to 127.0.0.1, an HTTP/HTTPS
// reverse proxy that routes requests by hostname to the correct local service
// port, and a file watcher that monitors the registry for changes and rebuilds
// the routing table automatically.
//
// The daemon is started by launchd (macOS) via socket activation and is not
// invoked directly by users. It is managed through the hidden "daemon" CLI
// command. When TLS is configured, the HTTP listener redirects all traffic to
// HTTPS, and the HTTPS listener terminates TLS using locally-trusted
// certificates from Outport's built-in CA.
package daemon

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/miekg/dns"
	"github.com/steveclarke/outport/internal/dashboard"
	"github.com/steveclarke/outport/internal/registry"
)

// DaemonConfig holds all configuration needed to start the daemon process.
// It is populated by the CLI's hidden "daemon" command and passed to New.
// The struct separates concerns so that the daemon package has no dependency
// on CLI flags, environment variables, or the settings package.
type DaemonConfig struct {
	// DNSAddr is the UDP address the DNS server listens on (e.g., "127.0.0.1:15353").
	// macOS resolver files point *.test queries to this address.
	DNSAddr string

	// DNSTTL is the time-to-live in seconds included in DNS responses. Higher
	// values reduce repeated lookups but delay hostname changes. Defaults to 60
	// if zero. Configurable via the global settings file.
	DNSTTL uint32

	// ProxyAddr is the TCP address the HTTP proxy listens on (e.g., ":80").
	// Only used as a fallback when HTTPListener is nil.
	ProxyAddr string

	// HTTPListener is a pre-bound TCP listener for the HTTP proxy, provided by
	// launchd socket activation on macOS. When set, ProxyAddr is ignored and the
	// proxy serves on this listener instead. This allows the daemon to bind to
	// privileged port 80 without running as root.
	HTTPListener net.Listener

	// HTTPSListener is a pre-bound TCP listener for the HTTPS proxy, also
	// provided by launchd socket activation. When set alongside TLSConfig, the
	// daemon serves TLS-terminated traffic on port 443.
	HTTPSListener net.Listener

	// TLSConfig holds the TLS configuration including the GetCertificate
	// callback that provisions per-hostname certificates on demand from
	// Outport's local CA. When nil, HTTPS is disabled and the HTTP proxy
	// serves traffic directly instead of redirecting to HTTPS.
	TLSConfig *tls.Config

	// RegistryPath is the absolute path to registry.json, the persistent store
	// of all project allocations. The daemon watches this file for changes and
	// automatically rebuilds its routing table when the file is modified.
	RegistryPath string

	// Version is the Outport version string (e.g., "0.23.0"), passed to the
	// dashboard so it can display the running version and expose it via the
	// /api/status endpoint for version-mismatch detection by CLI commands.
	Version string

	// HealthInterval controls how often the dashboard's health checker polls
	// service ports to determine if services are running. Only active when at
	// least one SSE client is connected. Defaults to 3 seconds if zero.
	// Configurable via the global settings file.
	HealthInterval time.Duration

	// NetworkInterface is the network interface name for LAN IP detection
	// (e.g., "en0"). When empty, the dashboard auto-detects the LAN interface.
	// Configurable via the global settings file.
	NetworkInterface string
}

// Daemon coordinates the three core servers (DNS, HTTP proxy, HTTPS proxy)
// and the registry file watcher. It owns their lifecycle: starting them
// concurrently in Run and shutting them all down when the context is
// cancelled or any server fails. The Daemon is the top-level orchestrator
// for the entire background process.
type Daemon struct {
	cfg      *DaemonConfig
	routes   *RouteTable
	dns      *dns.Server
	proxy    *http.Server
	tlsProxy *http.Server
}

// New creates and wires together all daemon components without starting them.
// It builds the proxy handler, dashboard handler, DNS server, and (optionally)
// the HTTPS server, connecting them through the shared RouteTable. When TLS is
// configured, the HTTP server redirects all requests to HTTPS; otherwise it
// serves proxy traffic directly. Call Run on the returned Daemon to start
// all servers.
func New(cfg *DaemonConfig) (*Daemon, error) {
	routes := &RouteTable{}
	proxyHandler := NewProxy(routes)

	httpsEnabled := cfg.TLSConfig != nil
	dashProvider := &routeTableProvider{routes: routes}

	healthInterval := cfg.HealthInterval
	if healthInterval == 0 {
		healthInterval = 3 * time.Second
	}
	dashHandler := dashboard.NewHandler(dashProvider, httpsEnabled, cfg.Version, healthInterval, cfg.NetworkInterface)
	proxyHandler.DashboardHandler = dashHandler

	routes.OnUpdate = func() {
		proxyHandler.ClearCache()
		dashHandler.OnRegistryUpdate()
	}

	dnsttl := cfg.DNSTTL
	if dnsttl == 0 {
		dnsttl = 60
	}
	dnsSrv := NewDNSServer(cfg.DNSAddr, dnsttl)

	var httpHandler http.Handler
	if cfg.TLSConfig != nil {
		httpHandler = http.HandlerFunc(redirectToHTTPS)
	} else {
		httpHandler = proxyHandler
	}

	httpSrv := &http.Server{
		Addr:              cfg.ProxyAddr,
		Handler:           httpHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	d := &Daemon{
		cfg:    cfg,
		routes: routes,
		dns:    dnsSrv,
		proxy:  httpSrv,
	}

	if cfg.TLSConfig != nil {
		d.tlsProxy = &http.Server{
			Handler:           withForwardedProto(proxyHandler),
			TLSConfig:         cfg.TLSConfig,
			ReadHeaderTimeout: 10 * time.Second,
		}
	}

	return d, nil
}

// withForwardedProto wraps a handler to set X-Forwarded-Proto: https on all
// requests, so backends behind the TLS proxy can detect the original scheme.
func withForwardedProto(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Forwarded-Proto", "https")
		h.ServeHTTP(w, r)
	})
}

// redirectToHTTPS sends a 307 Temporary Redirect from HTTP to the equivalent
// HTTPS URL. This is used as the HTTP server's handler when TLS is enabled,
// ensuring all browser traffic goes through the encrypted proxy.
func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	target := "https://" + r.Host + r.RequestURI
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

// Run starts all daemon servers concurrently and blocks until the context is
// cancelled or any server returns an error. On context cancellation (normal
// shutdown, e.g., launchd sending SIGTERM), it returns nil after gracefully
// stopping all servers. If any server fails unexpectedly, it shuts down the
// remaining servers and returns the error wrapped with context.
func (d *Daemon) Run(ctx context.Context) error {
	serverCount := 3 // DNS + HTTP + route watcher
	if d.tlsProxy != nil {
		serverCount = 4 // + HTTPS
	}
	errCh := make(chan error, serverCount)

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
		if d.cfg.HTTPListener != nil {
			err = d.proxy.Serve(d.cfg.HTTPListener)
		} else {
			err = d.proxy.ListenAndServe()
		}
		if err == http.ErrServerClosed {
			err = nil
		}
		errCh <- err
	}()

	// Start HTTPS proxy if TLS is configured
	if d.tlsProxy != nil {
		go func() {
			var err error
			if d.cfg.HTTPSListener != nil {
				err = d.tlsProxy.ServeTLS(d.cfg.HTTPSListener, "", "")
			} else {
				err = d.tlsProxy.ListenAndServeTLS("", "")
			}
			if err == http.ErrServerClosed {
				err = nil
			}
			errCh <- err
		}()
	}

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		d.shutdown()
		return nil
	case err := <-errCh:
		d.shutdown()
		return fmt.Errorf("daemon component failed: %w", err)
	}
}

// shutdown gracefully stops all daemon servers.
func (d *Daemon) shutdown() {
	_ = d.dns.Shutdown()
	d.proxy.Close()
	if d.tlsProxy != nil {
		d.tlsProxy.Close()
	}
}

// routeTableProvider adapts *RouteTable to the dashboard.AllocProvider
// interface. The dashboard package defines its own provider interface to avoid
// importing the daemon package, so this thin adapter bridges the two. It
// delegates directly to the RouteTable's thread-safe accessor methods.
type routeTableProvider struct {
	routes *RouteTable
}

// Allocations returns the current registry allocation data for all projects.
// It satisfies the dashboard.AllocProvider interface.
func (p *routeTableProvider) Allocations() map[string]registry.Allocation {
	return p.routes.Allocations()
}

// AllPorts returns the deduplicated list of all allocated ports across every
// registered project. It satisfies the dashboard.AllocProvider interface.
func (p *routeTableProvider) AllPorts() []int {
	return p.routes.AllPorts()
}
