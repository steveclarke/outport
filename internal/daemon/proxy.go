package daemon

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// ProxyHandler is the core HTTP handler for the daemon's reverse proxy. It
// inspects each incoming request's Host header, looks up the corresponding
// service port in the RouteTable, and forwards the request to
// http://localhost:<port> using a cached httputil.ReverseProxy. If no route
// matches the hostname, it renders a branded error page suggesting the user
// run "outport up". If the backend service is not running, it renders a
// different error page asking the user to start their app.
//
// The special hostname "outport.test" is intercepted before route lookup and
// delegated to the embedded dashboard handler.
type ProxyHandler struct {
	// routes is the thread-safe hostname-to-port mapping, rebuilt automatically
	// whenever the registry file changes on disk.
	routes *RouteTable

	// proxies is a concurrent cache of port -> *httputil.ReverseProxy. Each
	// proxy is created lazily on first request and reused for subsequent
	// requests to the same port. The cache is cleared whenever the route table
	// is updated, so stale proxies are never served.
	proxies sync.Map

	// DashboardHandler serves the web dashboard at outport.test. It is set by
	// the Daemon during initialization. When a request arrives for the hostname
	// "outport.test", it is handed off to this handler instead of being routed
	// through the normal proxy path.
	DashboardHandler http.Handler
}

// NewProxy creates a new ProxyHandler backed by the given RouteTable. The
// returned handler is safe for concurrent use and should be set as the HTTP
// server's handler. Go's httputil.ReverseProxy (1.20+) handles WebSocket
// upgrade requests transparently, so WebSocket connections through .test
// hostnames work without any special configuration.
func NewProxy(routes *RouteTable) *ProxyHandler {
	return &ProxyHandler{routes: routes}
}

// ClearCache discards all cached reverse proxies, forcing new ones to be
// created on the next request. Call this after route table updates.
func (p *ProxyHandler) ClearCache() {
	p.proxies.Clear()
}

// maxBackendConns caps the connections the proxy opens to one backend port at
// once. A browser sends a dev server's whole module graph (often hundreds of
// requests) over a single HTTP/2 connection. Opened as one TCP connection per
// request, that burst overflows the backend's listen queue (128 on macOS) and
// the kernel resets the extra connections. With the cap, the burst waits in
// the proxy for a free connection, as browsers do when speaking HTTP/1.1.
const maxBackendConns = 16

// getOrCreateProxy returns a cached reverse proxy for the given port, creating
// one if it does not already exist. The proxy targets http://localhost:<port>
// through its own transport, which holds at most maxBackendConns connections
// to the backend and keeps them alive for reuse. Created proxies persist until
// ClearCache is called (which happens on every route table update).
func (p *ProxyHandler) getOrCreateProxy(port int) *httputil.ReverseProxy {
	if v, ok := p.proxies.Load(port); ok {
		return v.(*httputil.ReverseProxy)
	}
	target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))
	proxy := httputil.NewSingleHostReverseProxy(target)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxConnsPerHost = maxBackendConns
	transport.MaxIdleConnsPerHost = maxBackendConns
	proxy.Transport = transport
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error: %s%s -> port %d: %v", r.Host, r.URL.Path, port, err)
		writeErrorPage(w, http.StatusBadGateway, stripPort(r.Host),
			"This app isn't running yet.<br>Start your app, then refresh this page.",
			"")
	}
	actual, _ := p.proxies.LoadOrStore(port, proxy)
	return actual.(*httputil.ReverseProxy)
}

// stripPort removes a trailing ":port" from a Host header value. Browsers
// include the port for non-standard ports.
func stripPort(host string) string {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}

// ServeHTTP handles every incoming HTTP request to the proxy. It strips the
// port from the Host header (browsers include it for non-standard ports),
// checks for the special "outport.test" dashboard hostname, looks up the
// target port in the route table, and forwards the request. If the hostname
// is not registered, a 502 error page is shown with instructions to run
// "outport up". If the backend service is unreachable (not running), the
// proxy's error handler renders a different 502 page asking the user to
// start their app.
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostname := stripPort(r.Host)

	// Dashboard intercept
	if hostname == "outport.test" && p.DashboardHandler != nil {
		p.DashboardHandler.ServeHTTP(w, r)
		return
	}

	rt, ok := p.routes.Lookup(hostname)
	if !ok {
		writeErrorPage(w, http.StatusBadGateway, hostname,
			"No project is configured for this hostname.<br>Add a matching hostname to your <code>outport.yml</code> and run:",
			`<div class="hint">outport up</div>`)
		return
	}

	// Rewrite Host header for tunnel routes so the backend sees the original
	// .test hostname. Preserve the original tunnel hostname in X-Forwarded-Host
	// so frameworks (Rails, Django, etc.) generate correct redirect URLs back
	// to the tunnel, not to the unreachable .test domain.
	if rt.HostOverride != "" {
		r.Header.Set("X-Forwarded-Host", hostname)
		r.Header.Set("X-Forwarded-Proto", "https")
		r.Host = rt.HostOverride
	}

	p.getOrCreateProxy(rt.Port).ServeHTTP(w, r)
}
