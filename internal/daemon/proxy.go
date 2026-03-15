package daemon

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// NewProxy creates an HTTP reverse proxy that routes by Host header.
// Go's httputil.ReverseProxy (1.20+) handles WebSocket upgrades transparently.
func NewProxy(routes *RouteTable) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hostname := r.Host
		if idx := strings.LastIndex(hostname, ":"); idx != -1 {
			hostname = hostname[:idx]
		}

		port, ok := routes.Lookup(hostname)
		if !ok {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "No project is configured for %s.\nRun `outport apply` with a matching hostname.\n", hostname)
			return
		}

		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "%s is not running.\nStart your app and try again.\n", hostname)
		}
		proxy.ServeHTTP(w, r)
	})
}
