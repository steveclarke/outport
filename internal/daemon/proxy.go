package daemon

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// ProxyHandler routes HTTP requests by Host header using cached reverse proxies.
type ProxyHandler struct {
	routes  *RouteTable
	proxies sync.Map // port (int) -> *httputil.ReverseProxy
}

// NewProxy creates an HTTP reverse proxy that routes by Host header.
// Go's httputil.ReverseProxy (1.20+) handles WebSocket upgrades transparently.
func NewProxy(routes *RouteTable) *ProxyHandler {
	return &ProxyHandler{routes: routes}
}

// ClearCache discards all cached reverse proxies, forcing new ones to be
// created on the next request. Call this after route table updates.
func (p *ProxyHandler) ClearCache() {
	p.proxies = sync.Map{}
}

func (p *ProxyHandler) getOrCreateProxy(port int) *httputil.ReverseProxy {
	if v, ok := p.proxies.Load(port); ok {
		return v.(*httputil.ReverseProxy)
	}
	target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))
	proxy := httputil.NewSingleHostReverseProxy(target)
	p.proxies.Store(port, proxy)
	return proxy
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostname := r.Host
	if idx := strings.LastIndex(hostname, ":"); idx != -1 {
		hostname = hostname[:idx]
	}

	port, ok := p.routes.Lookup(hostname)
	if !ok {
		writeErrorPage(w, http.StatusBadGateway, hostname,
			"No project is configured for this hostname.<br>Add a matching hostname to your <code>outport.yml</code> and run:",
			`<div class="hint">outport up</div>`)
		return
	}

	proxy := p.getOrCreateProxy(port)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		writeErrorPage(w, http.StatusBadGateway, hostname,
			"This app isn't running yet.<br>Start your app, then refresh this page.",
			"")
	}
	proxy.ServeHTTP(w, r)
}
