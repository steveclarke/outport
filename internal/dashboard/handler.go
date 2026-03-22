package dashboard

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"

	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/urlutil"
)

// AllocProvider gives access to the current registry state.
type AllocProvider interface {
	Allocations() map[string]registry.Allocation
	AllPorts() []int
}

// StatusResponse is the JSON structure returned by GET /api/status.
type StatusResponse struct {
	Version  string                  `json:"version"`
	Projects map[string]ProjectJSON `json:"projects"`
}

// ProjectJSON groups instances under a project name.
type ProjectJSON struct {
	Instances map[string]InstanceJSON `json:"instances"`
}

// InstanceJSON represents one project instance (e.g., "main" or "bxcf").
type InstanceJSON struct {
	ProjectDir string                 `json:"project_dir"`
	Services   map[string]ServiceJSON `json:"services"`
}

// ServiceJSON describes a single allocated service.
type ServiceJSON struct {
	Port     int    `json:"port"`
	EnvVar   string `json:"env_var,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	URL      string `json:"url,omitempty"`
	Up       *bool  `json:"up,omitempty"`
}

// portEntry maps a port back to the project, instance, and service that own it.
type portEntry struct {
	Project  string
	Instance string
	Service  string
}

// Handler serves the dashboard HTTP API and embedded static files.
type Handler struct {
	mux       *http.ServeMux
	provider  AllocProvider
	health    *HealthChecker
	sse       *Broadcaster
	https     bool
	version   string
	indexHTML []byte
	portIndex map[int]portEntry // port -> owning project/instance/service
}

// NewHandler creates a dashboard handler with all routes registered.
func NewHandler(provider AllocProvider, httpsEnabled bool, version string) *Handler {
	h := &Handler{
		mux:      http.NewServeMux(),
		provider: provider,
		sse:      NewBroadcaster(),
		https:    httpsEnabled,
		version:  version,
	}

	h.health = NewHealthChecker(provider.AllPorts, h.onHealthChange)
	h.indexHTML, _ = staticFiles.ReadFile("static/index.html")
	h.rebuildPortIndex()

	h.mux.HandleFunc("GET /api/status", h.handleStatus)
	h.mux.HandleFunc("GET /api/events", h.handleSSE)

	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("dashboard: embed sub: %v", err))
	}
	h.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	h.mux.HandleFunc("GET /", h.handleIndex)

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

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

// onHealthChange is the callback from the health checker when port statuses change.
// It maps port numbers back to project/instance/service names and broadcasts
// a health event to all SSE clients.
func (h *Handler) onHealthChange(changes map[int]bool) {
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

		svcNames := make([]string, 0, len(alloc.Ports))
		for svc := range alloc.Ports {
			svcNames = append(svcNames, svc)
		}
		sort.Strings(svcNames)

		for _, name := range svcNames {
			port := alloc.Ports[name]
			sj := ServiceJSON{Port: port}

			sj.EnvVar = alloc.EnvVars[name]

			hostname := alloc.Hostnames[name]
			protocol := alloc.Protocols[name]

			if hostname != "" {
				sj.Hostname = hostname
			}
			if protocol != "" {
				sj.Protocol = protocol
			}

			if u := urlutil.ServiceURL(protocol, hostname, port, h.https); u != "" {
				sj.URL = u
			}

			// Attach health status if we have it.
			if up, ok := healthStatus[port]; ok {
				upVal := up
				sj.Up = &upVal
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
