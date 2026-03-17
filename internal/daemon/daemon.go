package daemon

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/miekg/dns"
)

// DaemonConfig holds configuration for the daemon process.
type DaemonConfig struct {
	DNSAddr       string       // UDP address for DNS (e.g., "127.0.0.1:15353")
	ProxyAddr     string       // TCP address for HTTP proxy (e.g., ":80")
	HTTPListener  net.Listener // Pre-bound HTTP listener (launchd socket activation)
	HTTPSListener net.Listener // Pre-bound HTTPS listener (launchd socket activation)
	TLSConfig     *tls.Config  // TLS config with GetCertificate callback (nil = no HTTPS)
	RegistryPath  string       // Path to registry.json
}

// Daemon coordinates the DNS server, HTTP proxy, and route watcher.
type Daemon struct {
	cfg      *DaemonConfig
	routes   *RouteTable
	dns      *dns.Server
	proxy    *http.Server
	tlsProxy *http.Server
}

// New creates a new Daemon instance.
func New(cfg *DaemonConfig) (*Daemon, error) {
	routes := &RouteTable{}
	proxyHandler := NewProxy(routes)
	routes.OnUpdate = proxyHandler.ClearCache

	dnsSrv := NewDNSServer(cfg.DNSAddr)

	var httpHandler http.Handler
	if cfg.TLSConfig != nil {
		httpHandler = http.HandlerFunc(redirectToHTTPS)
	} else {
		httpHandler = proxyHandler
	}

	httpSrv := &http.Server{
		Addr:    cfg.ProxyAddr,
		Handler: httpHandler,
	}

	d := &Daemon{
		cfg:    cfg,
		routes: routes,
		dns:    dnsSrv,
		proxy:  httpSrv,
	}

	if cfg.TLSConfig != nil {
		d.tlsProxy = &http.Server{
			Handler:   withForwardedProto(proxyHandler),
			TLSConfig: cfg.TLSConfig,
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

func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	target := "https://" + r.Host + r.RequestURI
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

// Run starts the daemon and blocks until the context is cancelled.
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
