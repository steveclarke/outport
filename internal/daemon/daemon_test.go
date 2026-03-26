package daemon

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/registry"
)

func TestDaemonStartAndShutdown(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")

	// Write a minimal registry
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("app1", "main", registry.Allocation{
		ProjectDir: "/src/app1",
		Ports:      map[string]int{"web": 10001},
		Hostnames:  map[string]string{"web": "app1.test"},
	})
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(regPath, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Bind a random UDP port for DNS
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	dnsAddr := pc.LocalAddr().String()
	pc.Close() // free it so the DNS server can bind

	// Bind a random TCP port for the proxy
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}

	d, err := New(&DaemonConfig{
		DNSAddr:      dnsAddr,
		ProxyAddr:    ln.Addr().String(),
		RegistryPath: regPath,
		HTTPListener: ln,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Give the daemon a moment to start
	time.Sleep(200 * time.Millisecond)

	// Verify the route table was populated by the watcher
	r, ok := d.routes.Lookup("app1.test")
	if !ok {
		t.Fatal("expected app1.test route after daemon start")
	}
	if r.Port != 10001 {
		t.Fatalf("app1.test: got %d, want 10001", r.Port)
	}

	// Shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("daemon returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for daemon shutdown")
	}
}

func TestDaemonHTTPS(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")
	caCertPath := filepath.Join(dir, "ca-cert.pem")
	caKeyPath := filepath.Join(dir, "ca-key.pem")
	cacheDir := filepath.Join(dir, "certs")

	if err := certmanager.GenerateCA(caCertPath, caKeyPath); err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	store, err := certmanager.NewCertStore(caCertPath, caKeyPath, cacheDir)
	if err != nil {
		t.Fatalf("NewCertStore: %v", err)
	}

	// Start a backend HTTP server that echoes the X-Forwarded-Proto header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proto := r.Header.Get("X-Forwarded-Proto")
		w.Header().Set("X-Got-Proto", proto)
		_, _ = w.Write([]byte("hello from backend"))
	}))
	defer backend.Close()
	backendPort := backend.Listener.Addr().(*net.TCPAddr).Port

	// Write registry
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("app1", "main", registry.Allocation{
		ProjectDir: "/src/app1",
		Ports:      map[string]int{"web": backendPort},
		Hostnames:  map[string]string{"web": "app1.test"},
	})
	data, _ := json.MarshalIndent(reg, "", "  ")
	if err := os.WriteFile(regPath, data, 0644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	// Bind listeners
	httpLn, _ := net.Listen("tcp", "127.0.0.1:0")
	httpsLn, _ := net.Listen("tcp", "127.0.0.1:0")

	dnsPC, _ := net.ListenPacket("udp", "127.0.0.1:0")
	dnsAddr := dnsPC.LocalAddr().String()
	dnsPC.Close()

	d, err := New(&DaemonConfig{
		DNSAddr:       dnsAddr,
		ProxyAddr:     httpLn.Addr().String(),
		HTTPListener:  httpLn,
		HTTPSListener: httpsLn,
		TLSConfig:     &tls.Config{GetCertificate: store.GetCertificate},
		RegistryPath:  regPath,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// Load CA into client trust pool
	caCertPEM, _ := os.ReadFile(caCertPath)
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caCertPEM)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    pool,
				ServerName: "app1.test",
			},
		},
	}

	// Must set Host header so proxy can route by hostname
	req, _ := http.NewRequest("GET", "https://"+httpsLn.Addr().String()+"/", nil)
	req.Host = "app1.test"
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello from backend" {
		t.Errorf("body = %q, want %q", body, "hello from backend")
	}

	// Verify X-Forwarded-Proto was set by the TLS proxy
	if got := resp.Header.Get("X-Got-Proto"); got != "https" {
		t.Errorf("X-Forwarded-Proto = %q, want %q", got, "https")
	}

	cancel()
}

func TestDaemonHTTPRedirect(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")
	caCertPath := filepath.Join(dir, "ca-cert.pem")
	caKeyPath := filepath.Join(dir, "ca-key.pem")
	cacheDir := filepath.Join(dir, "certs")

	if err := certmanager.GenerateCA(caCertPath, caKeyPath); err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	store, _ := certmanager.NewCertStore(caCertPath, caKeyPath, cacheDir)

	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	data, _ := json.MarshalIndent(reg, "", "  ")
	if err := os.WriteFile(regPath, data, 0644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	httpLn, _ := net.Listen("tcp", "127.0.0.1:0")
	httpsLn, _ := net.Listen("tcp", "127.0.0.1:0")
	dnsPC, _ := net.ListenPacket("udp", "127.0.0.1:0")
	dnsAddr := dnsPC.LocalAddr().String()
	dnsPC.Close()

	d, _ := New(&DaemonConfig{
		DNSAddr:       dnsAddr,
		ProxyAddr:     httpLn.Addr().String(),
		HTTPListener:  httpLn,
		HTTPSListener: httpsLn,
		TLSConfig:     &tls.Config{GetCertificate: store.GetCertificate},
		RegistryPath:  regPath,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	req, _ := http.NewRequest("GET", "http://"+httpLn.Addr().String()+"/some/path", nil)
	req.Host = "myapp.test"
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusTemporaryRedirect)
	}
	location := resp.Header.Get("Location")
	if location != "https://myapp.test/some/path" {
		t.Errorf("Location = %q, want %q", location, "https://myapp.test/some/path")
	}

	cancel()
}

func TestDaemonHTTPProxyWithoutTLS(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proto := r.Header.Get("X-Forwarded-Proto")
		w.Header().Set("X-Got-Proto", proto)
		_, _ = w.Write([]byte("plain http"))
	}))
	defer backend.Close()
	backendPort := backend.Listener.Addr().(*net.TCPAddr).Port

	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("app1", "main", registry.Allocation{
		ProjectDir: "/src/app1",
		Ports:      map[string]int{"web": backendPort},
		Hostnames:  map[string]string{"web": "app1.test"},
	})
	data, _ := json.MarshalIndent(reg, "", "  ")
	if err := os.WriteFile(regPath, data, 0644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	httpLn, _ := net.Listen("tcp", "127.0.0.1:0")
	dnsPC, _ := net.ListenPacket("udp", "127.0.0.1:0")
	dnsAddr := dnsPC.LocalAddr().String()
	dnsPC.Close()

	// No TLSConfig — should proxy, not redirect
	d, _ := New(&DaemonConfig{
		DNSAddr:      dnsAddr,
		ProxyAddr:    httpLn.Addr().String(),
		HTTPListener: httpLn,
		RegistryPath: regPath,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	req, _ := http.NewRequest("GET", "http://"+httpLn.Addr().String()+"/", nil)
	req.Host = "app1.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "plain http" {
		t.Errorf("body = %q, want %q", body, "plain http")
	}

	// Verify X-Forwarded-Proto is NOT set on plain HTTP path
	if got := resp.Header.Get("X-Got-Proto"); got != "" {
		t.Errorf("X-Forwarded-Proto on plain HTTP = %q, want empty", got)
	}

	cancel()
}

func TestDaemonServesDashboard(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")

	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("myapp", "main", registry.Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"web": 10001},
		Hostnames:  map[string]string{"web": "myapp.test"},
	})
	writeRegistryJSON(t, regPath, reg)

	cfg := &DaemonConfig{
		DNSAddr:      "127.0.0.1:0",
		ProxyAddr:    "127.0.0.1:0",
		RegistryPath: regPath,
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("new daemon: %v", err)
	}

	// Use the proxy handler directly (avoid binding ports)
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "outport.test"
	w := httptest.NewRecorder()
	d.proxy.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Outport Dashboard") {
		t.Error("response should contain dashboard page title")
	}
}

func TestDaemonAPIStatus(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")

	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("myapp", "main", registry.Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"web": 10001},
		Hostnames:  map[string]string{"web": "myapp.test"},
	})
	writeRegistryJSON(t, regPath, reg)

	cfg := &DaemonConfig{
		DNSAddr:      "127.0.0.1:0",
		ProxyAddr:    "127.0.0.1:0",
		RegistryPath: regPath,
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("new daemon: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.Host = "outport.test"
	w := httptest.NewRecorder()
	d.proxy.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: got %q, want application/json", ct)
	}
}
