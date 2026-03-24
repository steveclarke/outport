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

// RouteTable is a thread-safe hostname -> port mapping.
type RouteTable struct {
	mu          sync.RWMutex
	routes      map[string]int
	allocations map[string]registry.Allocation // full registry data for dashboard
	ports       []int                          // deduplicated list of all allocated ports
	OnUpdate    func()                         // called after every Update, if non-nil
}

// Lookup returns the port for a hostname, or 0 if not found.
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

// UpdateWithAllocations swaps both the routing table and the full allocation
// data atomically, then fires the OnUpdate callback.
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

// AllPorts returns the cached deduplicated list of all allocated ports.
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

// Allocations returns a shallow copy of the full registry allocation data.
func (rt *RouteTable) Allocations() map[string]registry.Allocation {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	result := make(map[string]registry.Allocation, len(rt.allocations))
	for k, v := range rt.allocations {
		result[k] = v
	}
	return result
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

// WatchAndRebuild watches the registry file and rebuilds routes on changes.
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
