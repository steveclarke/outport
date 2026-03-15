package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
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
			if filepath.Base(event.Name) == base &&
				(event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
				rebuildFromFile(regPath, rt) // best-effort
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
	rt.Update(routes)
	return nil
}
