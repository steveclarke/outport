package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/outport-app/outport/internal/registry"
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
		Protocols:  map[string]string{"web": "http"},
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
		Listener:     ln,
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
	port, ok := d.routes.Lookup("app1.test")
	if !ok {
		t.Fatal("expected app1.test route after daemon start")
	}
	if port != 10001 {
		t.Fatalf("app1.test: got %d, want 10001", port)
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
