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
	routes      map[string]int                 // hostname (e.g., "myapp.test") -> port (e.g., 12345)
	allocations map[string]registry.Allocation // full registry data keyed by "project/instance", used by dashboard
	ports       []int                          // deduplicated list of all allocated ports across all projects
	// OnUpdate is an optional callback invoked after every route table update.
	// The daemon wires this to clear the proxy cache and notify the dashboard
	// of registry changes (triggering SSE events to connected browsers).
	OnUpdate func()
}

// Lookup returns the port mapped to the given hostname and true, or zero and
// false if no route exists for that hostname. This is the hot path called on
// every incoming proxy request, so it uses a read lock for maximum concurrency.
func (rt *RouteTable) Lookup(hostname string) (int, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	port, ok := rt.routes[hostname]
	return port, ok
}

// update swaps the routing table atomically and fires the OnUpdate callback.
// Used only in tests to set up minimal route tables without allocation data.
func (rt *RouteTable) update(routes map[string]int) {
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
func (rt *RouteTable) UpdateWithAllocations(routes map[string]int, allocs map[string]registry.Allocation) {
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

// BuildRoutes constructs a hostname-to-port routing map from the full registry.
// It iterates over every project allocation and maps each service's .test
// hostname to its allocated port. Only services that have a hostname configured
// in outport.yml (and therefore have an entry in alloc.Hostnames) produce a
// route. Services without hostnames are port-only and are not reachable
// through the proxy. The returned map is intended to be passed to
// RouteTable.UpdateWithAllocations.
func BuildRoutes(reg *registry.Registry) map[string]int {
	routes := make(map[string]int)
	for _, alloc := range reg.Projects {
		if alloc.Hostnames == nil {
			continue
		}
		for svcName, hostname := range alloc.Hostnames {
			routes[hostname] = alloc.Ports[svcName]
		}
	}
	return routes
}

// WatchAndRebuild performs an initial route build from the registry file, then
// watches the registry's parent directory for file changes. When registry.json
// is created or written, the routing table is rebuilt from the new file
// contents. When the tunnel state file changes, the OnUpdate callback is fired
// without rebuilding routes (so the dashboard can refresh tunnel status). The
// function blocks until the context is cancelled or a watcher error occurs.
// Errors during the initial load are returned immediately; errors during
// subsequent reloads are silently ignored (best-effort) to keep the daemon
// running with the last known good routes.
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
			} else if eventBase == tunnel.StateFilename {
				// Tunnel state changed — notify dashboard without rebuilding routes
				if rt.OnUpdate != nil {
					rt.OnUpdate()
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watcher error: %w", err)
		}
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
