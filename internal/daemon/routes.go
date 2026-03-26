package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/tunnel"
)

// route represents a single proxy route entry. Normal routes just carry a
// port; tunnel routes also carry a HostOverride so the proxy can rewrite the
// Host header to the original .test hostname before forwarding.
type route struct {
	Port         int
	HostOverride string // empty for normal routes, set for tunnel routes
}

// RouteTable is a thread-safe mapping from .test hostnames to local service
// ports. It is the central data structure shared between the proxy (which reads
// it on every request) and the file watcher (which writes it whenever the
// registry changes). In addition to the hostname-to-port routing map, the
// RouteTable also holds the full registry allocation data and a deduplicated
// port list, both of which are consumed by the dashboard for display and health
// checking.
//
// All reads and writes are protected by a sync.RWMutex, so multiple proxy
// goroutines can look up routes concurrently while the watcher goroutine
// atomically swaps in a new table.
type RouteTable struct {
	mu          sync.RWMutex
	routes      map[string]route               // hostname (e.g., "myapp.test") -> route
	allocations map[string]registry.Allocation // full registry data keyed by "project/instance", used by dashboard
	ports       []int                          // deduplicated list of all allocated ports across all projects
	// OnUpdate is an optional callback invoked after every route table update.
	// The daemon wires this to clear the proxy cache and notify the dashboard
	// of registry changes (triggering SSE events to connected browsers).
	OnUpdate func()
}

// Lookup returns the route mapped to the given hostname and true, or a zero
// route and false if no route exists for that hostname. This is the hot path
// called on every incoming proxy request, so it uses a read lock for maximum
// concurrency.
func (rt *RouteTable) Lookup(hostname string) (route, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	r, ok := rt.routes[hostname]
	return r, ok
}

// update swaps the routing table atomically and fires the OnUpdate callback.
// Used only in tests to set up minimal route tables without allocation data.
func (rt *RouteTable) update(routes map[string]route) {
	rt.mu.Lock()
	rt.routes = routes
	rt.mu.Unlock()
	if rt.OnUpdate != nil {
		rt.OnUpdate()
	}
}

// UpdateWithAllocations replaces both the hostname-to-port routing map and the
// full allocation data in a single atomic operation, then fires the OnUpdate
// callback. This is the primary update path used by the file watcher when the
// registry changes. The allocation data is stored so the dashboard can display
// project names, service names, and port assignments. A deduplicated port list
// is also computed and cached for the dashboard's health checker.
func (rt *RouteTable) UpdateWithAllocations(routes map[string]route, allocs map[string]registry.Allocation) {
	rt.mu.Lock()
	rt.routes = routes
	rt.allocations = allocs
	rt.ports = deduplicatePorts(allocs)
	rt.mu.Unlock()
	if rt.OnUpdate != nil {
		rt.OnUpdate()
	}
}

// AllPorts returns the cached deduplicated list of all allocated ports across
// every registered project and instance. The dashboard's health checker uses
// this to poll each port and determine which services are currently running.
func (rt *RouteTable) AllPorts() []int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.ports
}

// deduplicatePorts collects all unique ports from allocations.
func deduplicatePorts(allocs map[string]registry.Allocation) []int {
	seen := make(map[int]bool)
	var ports []int
	for _, alloc := range allocs {
		for _, port := range alloc.Ports {
			if !seen[port] {
				seen[port] = true
				ports = append(ports, port)
			}
		}
	}
	return ports
}

// Allocations returns a shallow copy of the full registry allocation data,
// keyed by "project/instance" (e.g., "myapp/main"). A copy is returned so
// that callers (primarily the dashboard) can iterate without holding the lock.
// The Allocation values themselves are not deep-copied, but they are treated
// as read-only by all consumers.
func (rt *RouteTable) Allocations() map[string]registry.Allocation {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	result := make(map[string]registry.Allocation, len(rt.allocations))
	for k, v := range rt.allocations {
		result[k] = v
	}
	return result
}

// BuildRoutes constructs a hostname-to-route mapping from the full registry.
// It iterates over every project allocation and maps each service's .test
// hostname to its allocated port. Only services that have a hostname configured
// in outport.yml (and therefore have an entry in alloc.Hostnames) produce a
// route. Services without hostnames are port-only and are not reachable
// through the proxy. Alias hostnames are also included, pointing to the same
// port as the primary hostname. The returned map is intended to be passed to
// RouteTable.UpdateWithAllocations.
func BuildRoutes(reg *registry.Registry) map[string]route {
	routes := make(map[string]route)
	for _, alloc := range reg.Projects {
		if alloc.Hostnames == nil {
			continue
		}
		for svcName, hostname := range alloc.Hostnames {
			routes[hostname] = route{Port: alloc.Ports[svcName]}
		}
		for svcName, svcAliases := range alloc.Aliases {
			for _, aliasHostname := range svcAliases {
				routes[aliasHostname] = route{Port: alloc.Ports[svcName]}
			}
		}
	}
	return routes
}

// BuildTunnelRoutes reads the tunnel state and returns HostOverride routes.
// Each tunnel URL hostname maps to the service's port with the original .test
// hostname as HostOverride, enabling the proxy to rewrite the Host header
// before forwarding to the local service.
func BuildTunnelRoutes(tunnelState *tunnel.TunnelState, allocs map[string]registry.Allocation) map[string]route {
	if tunnelState == nil || len(tunnelState.HostnameMap) == 0 {
		return nil
	}
	routes := make(map[string]route)
	for tunnelHostname, testHostname := range tunnelState.HostnameMap {
		// Find which port this .test hostname maps to
		for _, alloc := range allocs {
			// Check primary hostnames
			for svcName, h := range alloc.Hostnames {
				if h == testHostname {
					routes[tunnelHostname] = route{Port: alloc.Ports[svcName], HostOverride: testHostname}
				}
			}
			// Check aliases
			for svcName, svcAliases := range alloc.Aliases {
				for _, h := range svcAliases {
					if h == testHostname {
						routes[tunnelHostname] = route{Port: alloc.Ports[svcName], HostOverride: testHostname}
					}
				}
			}
		}
	}
	return routes
}

// MergeTunnelRoutes adds temporary HostOverride routes for active tunnels.
// These routes are merged into the existing route map without replacing it.
func (rt *RouteTable) MergeTunnelRoutes(tunnelRoutes map[string]route) {
	rt.mu.Lock()
	for hostname, r := range tunnelRoutes {
		rt.routes[hostname] = r
	}
	rt.mu.Unlock()
	if rt.OnUpdate != nil {
		rt.OnUpdate()
	}
}

// WatchAndRebuild performs an initial route build from the registry file, then
// watches the registry's parent directory for file changes. When registry.json
// is created or written, the routing table is rebuilt from the new file contents
// and any active tunnel routes are re-applied on top. When the tunnel state file
// changes, HostOverride routes are built from the tunnel state and merged into
// the route table (so the proxy can forward tunnel traffic to local services
// with the correct Host header). The function blocks until the context is
// cancelled or a watcher error occurs. Errors during the initial load are
// returned immediately; errors during subsequent reloads are silently ignored
// (best-effort) to keep the daemon running with the last known good routes.
func WatchAndRebuild(ctx context.Context, regPath string, rt *RouteTable) error {
	// Initial load
	if err := rebuildFromFile(regPath, rt); err != nil {
		return fmt.Errorf("initial route build: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	dir := filepath.Dir(regPath)
	base := filepath.Base(regPath)
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("watch directory %s: %w", dir, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			eventBase := filepath.Base(event.Name)
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}
			if eventBase == base {
				_ = rebuildFromFile(regPath, rt) // best-effort
				addTunnelRoutes(regPath, rt)     // re-apply tunnel routes after registry rebuild
			} else if eventBase == tunnel.StateFilename {
				addTunnelRoutes(regPath, rt)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watcher error: %w", err)
		}
	}
}

// addTunnelRoutes reads the tunnel state file and merges HostOverride routes
// into the route table. If the tunnel state file is missing or stale, this is
// a no-op. Called both when the tunnel state file changes and after registry
// rebuilds (to re-apply tunnel routes that would otherwise be lost).
//
// Known limitation: fsnotify does not fire events for file removals, so when
// tunnels stop and the state file is deleted, these routes linger until the
// next registry rebuild (e.g., any "outport up" or "outport down").
func addTunnelRoutes(regPath string, rt *RouteTable) {
	statePath := filepath.Join(filepath.Dir(regPath), tunnel.StateFilename)
	state, err := tunnel.ReadState(statePath)
	if err != nil || state == nil {
		return
	}
	tunnelRoutes := BuildTunnelRoutes(state, rt.Allocations())
	if len(tunnelRoutes) > 0 {
		rt.MergeTunnelRoutes(tunnelRoutes)
	}
}

// rebuildFromFile reads the registry JSON file, parses it, builds a new routing
// map, and atomically swaps it into the RouteTable along with the full
// allocation data. If the file does not exist, the function returns nil without
// modifying the existing routes, allowing the daemon to start before any
// projects have been registered.
func rebuildFromFile(regPath string, rt *RouteTable) error {
	data, err := os.ReadFile(regPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // keep existing routes
		}
		return err
	}
	var reg registry.Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return err
	}
	routes := BuildRoutes(&reg)
	rt.UpdateWithAllocations(routes, reg.Projects)
	return nil
}
