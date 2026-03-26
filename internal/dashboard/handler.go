// Package dashboard implements the embedded web dashboard served at outport.test.
// It provides a JSON API for querying project and service status, server-sent events
// (SSE) for real-time updates, and serves the embedded static frontend files. The
// dashboard is the primary visual interface for monitoring all registered projects,
// their port allocations, health status, and tunnel URLs. The daemon's proxy handler
// intercepts requests to outport.test and delegates them to this package's Handler.
package dashboard

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"slices"
	"sort"
	"time"

	"github.com/steveclarke/outport/internal/lanip"
	"github.com/steveclarke/outport/internal/qrcode"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/tunnel"
	"github.com/steveclarke/outport/internal/urlutil"
)

// AllocProvider is the interface through which the dashboard accesses the current
// registry state. The daemon's route table implements this interface, allowing the
// dashboard to read all project allocations and the complete list of allocated ports
// without importing the registry package directly.
type AllocProvider interface {
	// Allocations returns every registered project/instance allocation, keyed by
	// "{project}/{instance}" (e.g., "myapp/main"). Each allocation contains the
	// port mappings, hostnames, and env var names for that instance's services.
	Allocations() map[string]registry.Allocation

	// AllPorts returns a flat list of every allocated port number across all
	// projects and instances. The health checker uses this list to know which
	// ports to probe for liveness.
	AllPorts() []int
}

// StatusResponse is the top-level JSON structure returned by GET /api/status.
// It contains the daemon version, the local network IP address (for LAN access),
// and a map of all registered projects with their instances and services.
type StatusResponse struct {
	// Version is the running daemon's version string (e.g., "0.8.0").
	Version string `json:"version"`

	// Projects maps project names to their ProjectJSON data. Each project
	// may contain multiple instances (main checkout plus worktrees/clones).
	Projects map[string]ProjectJSON `json:"projects"`

	// LANIP is the machine's local network IP address, used by the dashboard
	// to display LAN-accessible URLs for mobile device testing. Empty when
	// the LAN IP cannot be detected.
	LANIP string `json:"lan_ip,omitempty"`
}

// ProjectJSON groups all instances of a single project. A project typically has
// a "main" instance and zero or more worktree/clone instances identified by
// short codes (e.g., "bxcf").
type ProjectJSON struct {
	// Instances maps instance names to their InstanceJSON data.
	Instances map[string]InstanceJSON `json:"instances"`
}

// InstanceJSON represents one project instance (e.g., "main" or "bxcf"). Each
// instance corresponds to a separate checkout of the project on disk, such as
// the primary checkout ("main") or a git worktree.
type InstanceJSON struct {
	// ProjectDir is the absolute filesystem path to this instance's checkout directory.
	ProjectDir string `json:"project_dir"`

	// Services maps service names (as defined in outport.yml) to their ServiceJSON data.
	Services map[string]ServiceJSON `json:"services"`
}

// AliasJSON describes a single hostname alias for a service.
type AliasJSON struct {
	Hostname  string `json:"hostname"`
	URL       string `json:"url,omitempty"`
	TunnelURL string `json:"tunnel_url,omitempty"`
}

// ServiceJSON describes a single allocated service within a project instance.
// It carries everything the dashboard needs to display: the port number, the
// browsable URL, whether the service process is currently running, and any
// active tunnel URL for external access.
type ServiceJSON struct {
	// Port is the deterministically allocated TCP port for this service.
	Port int `json:"port"`

	// EnvVar is the environment variable name that holds this service's port
	// (e.g., "PORT" or "VITE_PORT"). Empty for services that don't export a variable.
	EnvVar string `json:"env_var,omitempty"`

	// Hostname is the .test domain assigned to this service (e.g., "myapp.test").
	// Empty for infrastructure services that don't have a browsable hostname.
	Hostname string `json:"hostname,omitempty"`

	// URL is the fully qualified browsable URL (e.g., "https://myapp.test").
	// Constructed from the hostname and the current HTTPS setting. Empty when
	// the service has no hostname.
	URL string `json:"url,omitempty"`

	// Up indicates whether the service is currently accepting TCP connections on
	// its port. nil means health status is not yet known (no health check has
	// run); a pointer is used so the JSON field is omitted until a check occurs.
	Up *bool `json:"up,omitempty"`

	// TunnelURL is the public URL provided by a tunnel provider (e.g., Cloudflare)
	// for this service. Empty when no tunnel is active.
	TunnelURL string `json:"tunnel_url,omitempty"`

	// Aliases maps alias keys to their AliasJSON data. Each alias represents an
	// additional hostname that routes to this service's port.
	Aliases map[string]AliasJSON `json:"aliases,omitempty"`
}

// portEntry maps a port back to the project, instance, and service that own it.
type portEntry struct {
	Project  string
	Instance string
	Service  string
}

// Handler serves the dashboard HTTP API and embedded static files at outport.test.
// It implements http.Handler and is mounted by the daemon's proxy. The handler manages
// the health checker (which periodically probes ports for liveness), the SSE broadcaster
// (which pushes real-time updates to connected dashboard clients), and caches for LAN IP
// and tunnel state to avoid repeated lookups on every API request.
type Handler struct {
	mux              *http.ServeMux
	provider         AllocProvider
	health           *HealthChecker
	sse              *Broadcaster
	https            bool
	version          string
	networkInterface string                       // configured LAN interface override
	indexHTML         []byte
	portIndex        map[int]portEntry              // port -> owning project/instance/service
	cachedLANIP      string                         // cached LAN IP string
	cachedTunnel     map[string]map[string]string   // cached tunnel state
}

// NewHandler creates a dashboard handler with all HTTP routes registered. It sets up
// the health checker (which probes allocated ports at the given interval), the SSE
// broadcaster, and pre-caches the index.html file, LAN IP, and tunnel state. The
// registered routes are:
//   - GET /api/status  — full JSON status of all projects, instances, and services
//   - GET /api/version — lightweight version-only endpoint for CLI version checks
//   - GET /api/events  — SSE stream for real-time registry and health updates
//   - GET /api/qr      — SVG QR code generator for a given URL parameter
//   - GET /static/...  — embedded CSS, JS, and image assets
//   - GET /            — the dashboard HTML page
func NewHandler(provider AllocProvider, httpsEnabled bool, version string, healthInterval time.Duration, networkInterface string) *Handler {
	h := &Handler{
		mux:              http.NewServeMux(),
		provider:         provider,
		sse:              NewBroadcaster(),
		https:            httpsEnabled,
		version:          version,
		networkInterface: networkInterface,
	}

	h.health = NewHealthChecker(provider.AllPorts, healthInterval, h.onHealthChange)
	h.indexHTML, _ = staticFiles.ReadFile("static/index.html")
	h.rebuildPortIndex()
	h.refreshCaches()

	h.mux.HandleFunc("GET /api/status", h.handleStatus)
	h.mux.HandleFunc("GET /api/version", h.handleVersion)
	h.mux.HandleFunc("GET /api/events", h.handleSSE)
	h.mux.HandleFunc("GET /api/qr", h.handleQR)

	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("dashboard: embed sub: %v", err))
	}
	h.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	h.mux.HandleFunc("GET /", h.handleIndex)

	return h
}

// ServeHTTP dispatches incoming HTTP requests to the appropriate handler via the
// internal multiplexer. This makes Handler a valid http.Handler that the daemon's
// proxy can delegate to when it receives a request for outport.test.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// handleIndex serves the dashboard's main HTML page from the embedded static files.
func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(h.indexHTML)
}

// handleStatus returns the current status of all projects as JSON.
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := h.buildStatus()
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		http.Error(w, "encoding status: "+err.Error(), http.StatusInternalServerError)
	}
}

// handleVersion returns only the daemon version, used by the CLI to detect
// version mismatches without the overhead of building the full status response.
func (h *Handler) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Version string `json:"version"`
	}{Version: h.version})
}

// handleQR returns an SVG QR code for the given URL parameter.
func (h *Handler) handleQR(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "missing url parameter", http.StatusBadRequest)
		return
	}
	svg, err := qrcode.SVG(url)
	if err != nil {
		http.Error(w, "generating QR code: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	fmt.Fprint(w, svg)
}

// handleSSE streams server-sent events to the client.
func (h *Handler) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.sse.Subscribe()
	defer h.sse.Unsubscribe(ch)

	if h.sse.ClientCount() == 1 {
		h.health.Start()
	}

	resp := h.buildStatus()
	data, _ := json.Marshal(resp)
	writeSSE(w, "registry", string(data))
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			if h.sse.ClientCount() <= 1 {
				h.health.Stop()
			}
			return
		case evt := <-ch:
			writeSSE(w, evt.Type, evt.Data)
			flusher.Flush()
		}
	}
}

// OnRegistryUpdate is called by the daemon when the registry file changes.
// Sends the full state as a registry event. The next health tick (3s) will
// pick up port status for any newly registered services.
func (h *Handler) OnRegistryUpdate() {
	h.rebuildPortIndex()
	h.refreshCaches()
	if h.sse.ClientCount() == 0 {
		return
	}
	resp := h.buildStatus()
	data, _ := json.Marshal(resp)
	h.sse.Send(Event{Type: "registry", Data: string(data)})
}

// rebuildPortIndex reconstructs the port -> (project, instance, service) reverse index
// from the current provider allocations.
func (h *Handler) rebuildPortIndex() {
	allocs := h.provider.Allocations()
	idx := make(map[int]portEntry, len(allocs)*2)
	for key, alloc := range allocs {
		project, instance := registry.ParseKey(key)
		for svc, port := range alloc.Ports {
			idx[port] = portEntry{
				Project:  project,
				Instance: instance,
				Service:  svc,
			}
		}
	}
	h.portIndex = idx
}

// refreshCaches updates the cached LAN IP and tunnel state.
// Called on startup and on every registry/tunnel file change.
func (h *Handler) refreshCaches() {
	if ip, err := lanip.Detect(h.networkInterface); err == nil {
		h.cachedLANIP = ip.String()
	}
	statePath, err := tunnel.DefaultStatePath()
	if err != nil {
		h.cachedTunnel = nil
		return
	}
	state, err := tunnel.ReadState(statePath)
	if err != nil || state == nil {
		h.cachedTunnel = nil
		return
	}
	h.cachedTunnel = state.Tunnels
}

// onHealthChange is the callback from the health checker when port statuses change.
// It maps port numbers back to project/instance/service names and broadcasts
// a health event to all SSE clients. It also refreshes the tunnel cache on each
// tick so stale tunnel URLs (from an exited outport share process) disappear
// within one health interval.
func (h *Handler) onHealthChange(changes map[int]bool) {
	// Refresh tunnel state on each health tick so stale tunnels disappear within
	// one interval after outport share exits. If the tunnel state changed, also
	// send a registry event so the dashboard UI updates immediately.
	oldTunnel := h.cachedTunnel
	h.refreshCaches()
	tunnelChanged := (oldTunnel == nil) != (h.cachedTunnel == nil) || len(oldTunnel) != len(h.cachedTunnel)
	if tunnelChanged {
		resp := h.buildStatus()
		data, _ := json.Marshal(resp)
		h.sse.Send(Event{Type: "registry", Data: string(data)})
	}

	type changeEntry struct {
		Project  string `json:"project"`
		Instance string `json:"instance"`
		Service  string `json:"service"`
		Up       bool   `json:"up"`
	}

	var entries []changeEntry
	for port, up := range changes {
		if pe, ok := h.portIndex[port]; ok {
			entries = append(entries, changeEntry{
				Project:  pe.Project,
				Instance: pe.Instance,
				Service:  pe.Service,
				Up:       up,
			})
		}
	}

	// Sort for stable output.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Project != entries[j].Project {
			return entries[i].Project < entries[j].Project
		}
		if entries[i].Instance != entries[j].Instance {
			return entries[i].Instance < entries[j].Instance
		}
		return entries[i].Service < entries[j].Service
	})

	data, _ := json.Marshal(entries)
	h.sse.Send(Event{Type: "health", Data: string(data)})
}

// buildStatus constructs the full status response from the provider and health checker.
func (h *Handler) buildStatus() StatusResponse {
	allocs := h.provider.Allocations()
	healthStatus := h.health.CurrentStatus()

	resp := StatusResponse{
		Version:  h.version,
		Projects: make(map[string]ProjectJSON),
	}

	resp.LANIP = h.cachedLANIP
	tunnelState := h.cachedTunnel

	for key, alloc := range allocs {
		project, instance := registry.ParseKey(key)

		pj, ok := resp.Projects[project]
		if !ok {
			pj = ProjectJSON{Instances: make(map[string]InstanceJSON)}
		}

		ij := InstanceJSON{
			ProjectDir: alloc.ProjectDir,
			Services:   make(map[string]ServiceJSON),
		}

		svcNames := slices.Sorted(maps.Keys(alloc.Ports))

		for _, name := range svcNames {
			port := alloc.Ports[name]
			sj := ServiceJSON{Port: port}

			sj.EnvVar = alloc.EnvVars[name]

			hostname := alloc.Hostnames[name]

			if hostname != "" {
				sj.Hostname = hostname
			}

			if u := urlutil.ServiceURL(hostname, port, h.https); u != "" {
				sj.URL = u
			}

			// Attach health status if we have it.
			if up, ok := healthStatus[port]; ok {
				upVal := up
				sj.Up = &upVal
			}

			if tunnelState != nil {
				if svcTunnels, ok := tunnelState[key]; ok {
					if turl, ok := svcTunnels[name]; ok {
						sj.TunnelURL = turl
					}
				}
			}

			// Aliases
			if svcAliases, ok := alloc.Aliases[name]; ok && len(svcAliases) > 0 {
				aliasMap := make(map[string]AliasJSON, len(svcAliases))
				for aliasKey, aliasHostname := range svcAliases {
					aj := AliasJSON{
						Hostname: aliasHostname,
					}
					if u := urlutil.ServiceURL(aliasHostname, port, h.https); u != "" {
						aj.URL = u
					}
					if tunnelState != nil {
						if svcTunnels, ok := tunnelState[key]; ok {
							tunnelKey := name + "/alias/" + aliasKey
							if turl, ok := svcTunnels[tunnelKey]; ok {
								aj.TunnelURL = turl
							}
						}
					}
					aliasMap[aliasKey] = aj
				}
				sj.Aliases = aliasMap
			}

			ij.Services[name] = sj
		}

		pj.Instances[instance] = ij
		resp.Projects[project] = pj
	}

	return resp
}

// writeSSE writes one SSE event to the writer in the standard format.
func writeSSE(w http.ResponseWriter, eventType, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
}
