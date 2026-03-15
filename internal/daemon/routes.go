package daemon

import (
	"sync"

	"github.com/outport-app/outport/internal/registry"
)

// RouteTable is a thread-safe hostname -> port mapping.
type RouteTable struct {
	mu     sync.RWMutex
	routes map[string]int
}

// Lookup returns the port for a hostname, or 0 if not found.
func (rt *RouteTable) Lookup(hostname string) (int, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	port, ok := rt.routes[hostname]
	return port, ok
}

// Update swaps the routing table atomically.
func (rt *RouteTable) Update(routes map[string]int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.routes = routes
}

// BuildRoutes constructs a hostname -> port routing table from the registry.
// Only services with protocol "http" or "https" are included.
func BuildRoutes(reg *registry.Registry) map[string]int {
	routes := make(map[string]int)
	for _, alloc := range reg.Projects {
		if alloc.Hostnames == nil {
			continue
		}
		for svcName, hostname := range alloc.Hostnames {
			proto := alloc.Protocols[svcName]
			if proto == "http" || proto == "https" {
				routes[hostname] = alloc.Ports[svcName]
			}
		}
	}
	return routes
}
