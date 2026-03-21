package dashboard

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"

	"github.com/outport-app/outport/internal/registry"
)

// AllocProvider gives access to the current registry state.
type AllocProvider interface {
	Allocations() map[string]registry.Allocation
	AllPorts() []int
}

// StatusResponse is the JSON structure returned by GET /api/status.
type StatusResponse struct {
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
	Hostname string `json:"hostname,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	URL      string `json:"url,omitempty"`
	Up       *bool  `json:"up,omitempty"`
}

// Handler serves the dashboard HTTP API and embedded static files.
type Handler struct {
	mux      *http.ServeMux
	provider AllocProvider
	health   *HealthChecker
	sse      *Broadcaster
	https    bool
}

// NewHandler creates a dashboard handler with all routes registered.
func NewHandler(provider AllocProvider, httpsEnabled bool) *Handler {
	h := &Handler{
		mux:      http.NewServeMux(),
		provider: provider,
		sse:      NewBroadcaster(),
		https:    httpsEnabled,
	}

	h.health = NewHealthChecker(provider.AllPorts, h.onHealthChange)

	h.mux.HandleFunc("GET /api/status", h.handleStatus)
	h.mux.HandleFunc("GET /api/events", h.handleSSE)

	// Serve embedded static files.
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

// handleIndex serves the embedded index.html for the root path.
func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
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

	// Start health checker if this is the first client.
	if h.sse.ClientCount() == 1 {
		h.health.Start()
	}

	// Send initial full state.
	resp := h.buildStatus()
	data, _ := json.Marshal(resp)
	writeSSE(w, "registry", string(data))
	flusher.Flush()

	// Stream events until the client disconnects.
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Stop health checker if this was the last client.
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
// If SSE clients are connected, it triggers an immediate health check and
// sends the full state as a registry event.
func (h *Handler) OnRegistryUpdate() {
	if h.sse.ClientCount() == 0 {
		return
	}
	h.health.CheckNow()
	resp := h.buildStatus()
	data, _ := json.Marshal(resp)
	h.sse.Send(Event{Type: "registry", Data: string(data)})
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

	// Build a port -> (project, instance, service) reverse lookup.
	allocs := h.provider.Allocations()
	portIndex := make(map[int]changeEntry)
	for key, alloc := range allocs {
		project, instance := registry.ParseKey(key)
		for svc, port := range alloc.Ports {
			portIndex[port] = changeEntry{
				Project:  project,
				Instance: instance,
				Service:  svc,
			}
		}
	}

	var entries []changeEntry
	for port, up := range changes {
		if entry, ok := portIndex[port]; ok {
			entry.Up = up
			entries = append(entries, entry)
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

		// Collect and sort service names for stable output.
		svcNames := make([]string, 0, len(alloc.Ports))
		for svc := range alloc.Ports {
			svcNames = append(svcNames, svc)
		}
		sort.Strings(svcNames)

		for _, svc := range svcNames {
			port := alloc.Ports[svc]
			sj := ServiceJSON{Port: port}

			hostname := alloc.Hostnames[svc]
			protocol := alloc.Protocols[svc]

			if hostname != "" {
				sj.Hostname = hostname
			}
			if protocol != "" {
				sj.Protocol = protocol
			}

			// Build URL for web services (those with hostname + http/https protocol).
			if hostname != "" && (protocol == "http" || protocol == "https") {
				scheme := "http"
				if h.https {
					scheme = "https"
				}
				sj.URL = fmt.Sprintf("%s://%s", scheme, hostname)
			}

			// Attach health status if we have it.
			if up, ok := healthStatus[port]; ok {
				upVal := up
				sj.Up = &upVal
			}

			ij.Services[svc] = sj
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
