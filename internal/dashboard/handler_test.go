package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/outport-app/outport/internal/registry"
)

type mockAllocProvider struct {
	allocs map[string]registry.Allocation
}

func (m *mockAllocProvider) Allocations() map[string]registry.Allocation {
	return m.allocs
}

func (m *mockAllocProvider) AllPorts() []int {
	var ports []int
	for _, a := range m.allocs {
		for _, p := range a.Ports {
			ports = append(ports, p)
		}
	}
	return ports
}

func TestHandlerAPIStatus(t *testing.T) {
	provider := &mockAllocProvider{
		allocs: map[string]registry.Allocation{
			"myapp/main": {
				ProjectDir: "/home/dev/myapp",
				Ports:      map[string]int{"web": 13000, "postgres": 15432},
				Hostnames:  map[string]string{"web": "myapp.test"},
				Protocols:  map[string]string{"web": "http"},
			},
		},
	}

	h := NewHandler(provider, true)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("content-type: got %q, want %q", ct, "application/json")
	}

	var resp StatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	proj, ok := resp.Projects["myapp"]
	if !ok {
		t.Fatal("missing project 'myapp'")
	}

	inst, ok := proj.Instances["main"]
	if !ok {
		t.Fatal("missing instance 'main'")
	}

	if inst.ProjectDir != "/home/dev/myapp" {
		t.Errorf("project_dir: got %q, want %q", inst.ProjectDir, "/home/dev/myapp")
	}

	web, ok := inst.Services["web"]
	if !ok {
		t.Fatal("missing service 'web'")
	}
	if web.Port != 13000 {
		t.Errorf("web port: got %d, want %d", web.Port, 13000)
	}
	if web.Hostname != "myapp.test" {
		t.Errorf("web hostname: got %q, want %q", web.Hostname, "myapp.test")
	}
	if web.URL != "https://myapp.test" {
		t.Errorf("web url: got %q, want %q", web.URL, "https://myapp.test")
	}

	pg, ok := inst.Services["postgres"]
	if !ok {
		t.Fatal("missing service 'postgres'")
	}
	if pg.Port != 15432 {
		t.Errorf("postgres port: got %d, want %d", pg.Port, 15432)
	}
	if pg.Hostname != "" {
		t.Errorf("postgres hostname: got %q, want empty", pg.Hostname)
	}
	if pg.URL != "" {
		t.Errorf("postgres url: got %q, want empty", pg.URL)
	}
}

func TestHandlerAPIStatusMultipleInstances(t *testing.T) {
	provider := &mockAllocProvider{
		allocs: map[string]registry.Allocation{
			"myapp/main": {
				ProjectDir: "/home/dev/myapp",
				Ports:      map[string]int{"web": 13000},
				Hostnames:  map[string]string{"web": "myapp.test"},
				Protocols:  map[string]string{"web": "http"},
			},
			"myapp/bxcf": {
				ProjectDir: "/home/dev/myapp-bxcf",
				Ports:      map[string]int{"web": 13100},
				Hostnames:  map[string]string{"web": "myapp-bxcf.test"},
				Protocols:  map[string]string{"web": "http"},
			},
		},
	}

	h := NewHandler(provider, false)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp StatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	proj, ok := resp.Projects["myapp"]
	if !ok {
		t.Fatal("missing project 'myapp'")
	}

	if len(proj.Instances) != 2 {
		t.Fatalf("instance count: got %d, want 2", len(proj.Instances))
	}

	mainInst, ok := proj.Instances["main"]
	if !ok {
		t.Fatal("missing instance 'main'")
	}
	if mainInst.ProjectDir != "/home/dev/myapp" {
		t.Errorf("main project_dir: got %q, want %q", mainInst.ProjectDir, "/home/dev/myapp")
	}
	if mainInst.Services["web"].URL != "http://myapp.test" {
		t.Errorf("main web url: got %q, want %q", mainInst.Services["web"].URL, "http://myapp.test")
	}

	bxcfInst, ok := proj.Instances["bxcf"]
	if !ok {
		t.Fatal("missing instance 'bxcf'")
	}
	if bxcfInst.ProjectDir != "/home/dev/myapp-bxcf" {
		t.Errorf("bxcf project_dir: got %q, want %q", bxcfInst.ProjectDir, "/home/dev/myapp-bxcf")
	}
	if bxcfInst.Services["web"].URL != "http://myapp-bxcf.test" {
		t.Errorf("bxcf web url: got %q, want %q", bxcfInst.Services["web"].URL, "http://myapp-bxcf.test")
	}
}

func TestHandlerServesIndex(t *testing.T) {
	provider := &mockAllocProvider{
		allocs: map[string]registry.Allocation{},
	}

	h := NewHandler(provider, false)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("content-type: got %q, want %q", ct, "text/html; charset=utf-8")
	}

	body := rec.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestHandlerStatusHTTPScheme(t *testing.T) {
	provider := &mockAllocProvider{
		allocs: map[string]registry.Allocation{
			"myapp/main": {
				ProjectDir: "/home/dev/myapp",
				Ports:      map[string]int{"web": 13000},
				Hostnames:  map[string]string{"web": "myapp.test"},
				Protocols:  map[string]string{"web": "http"},
			},
		},
	}

	// HTTPS disabled — URL should use http://
	h := NewHandler(provider, false)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var resp StatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	url := resp.Projects["myapp"].Instances["main"].Services["web"].URL
	if url != "http://myapp.test" {
		t.Errorf("url with https=false: got %q, want %q", url, "http://myapp.test")
	}
}

func TestHandlerStatusNoURLForNonWebServices(t *testing.T) {
	provider := &mockAllocProvider{
		allocs: map[string]registry.Allocation{
			"myapp/main": {
				ProjectDir: "/home/dev/myapp",
				Ports:      map[string]int{"redis": 16379},
				Hostnames:  map[string]string{},
				Protocols:  map[string]string{},
			},
		},
	}

	h := NewHandler(provider, true)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var resp StatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	svc := resp.Projects["myapp"].Instances["main"].Services["redis"]
	if svc.URL != "" {
		t.Errorf("redis url: got %q, want empty", svc.URL)
	}
	if svc.Hostname != "" {
		t.Errorf("redis hostname: got %q, want empty", svc.Hostname)
	}
}
