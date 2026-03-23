package daemon

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/websocket"
)

func backendPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return port
}

func TestProxyRoutesToBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello from backend"))
	}))
	defer backend.Close()

	port := backendPort(t, backend)
	routes := &RouteTable{}
	routes.update(map[string]int{"myapp.test": port})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "myapp.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello from backend" {
		t.Errorf("got %q, want %q", body, "hello from backend")
	}
}

func TestProxyPreservesPath(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()

	port := backendPort(t, backend)
	routes := &RouteTable{}
	routes.update(map[string]int{"myapp.test": port})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/users", nil)
	req.Host = "myapp.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	if gotPath != "/api/v1/users" {
		t.Errorf("path: got %q, want /api/v1/users", gotPath)
	}
}

func TestProxyUnknownHostReturnsError(t *testing.T) {
	routes := &RouteTable{}
	routes.update(map[string]int{})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "unknown.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status: got %d, want 502", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "unknown.test") {
		t.Errorf("body should mention the hostname, got %q", body)
	}
}

func TestProxyBackendDownReturnsError(t *testing.T) {
	routes := &RouteTable{}
	routes.update(map[string]int{"myapp.test": 59999})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "myapp.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status: got %d, want 502", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "myapp.test") {
		t.Errorf("body should mention the hostname, got %q", body)
	}
}

func TestProxyStripsPortFromHost(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()

	port := backendPort(t, backend)
	routes := &RouteTable{}
	routes.update(map[string]int{"myapp.test": port})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	// Host header with port should still match
	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "myapp.test:8080"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestProxyRoutesOutportTestToDashboard(t *testing.T) {
	routes := &RouteTable{}
	routes.update(map[string]int{})

	dashHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("dashboard"))
	})

	proxy := NewProxy(routes)
	proxy.DashboardHandler = dashHandler

	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "outport.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "dashboard" {
		t.Errorf("got %q, want %q", body, "dashboard")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestProxyWebSocketUpgrade(t *testing.T) {
	// Backend that speaks WebSocket
	backend := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		var msg string
		if err := websocket.Message.Receive(ws, &msg); err != nil {
			return
		}
		_ = websocket.Message.Send(ws, "echo: "+msg)
	}))
	defer backend.Close()

	port := backendPort(t, backend)
	routes := &RouteTable{}
	routes.update(map[string]int{"myapp.test": port})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	// Connect WebSocket through the proxy.
	// Location uses myapp.test so the handshake sends Host: myapp.test.
	// Dial to the proxy address directly and perform the WS handshake.
	proxyURL, _ := url.Parse(srv.URL)

	config, err := websocket.NewConfig("ws://myapp.test/", "http://myapp.test/")
	if err != nil {
		t.Fatalf("ws config: %v", err)
	}

	conn, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}

	ws, err := websocket.NewClient(config, conn)
	if err != nil {
		conn.Close()
		t.Fatalf("ws handshake: %v", err)
	}
	defer ws.Close()

	if err := websocket.Message.Send(ws, "hello"); err != nil {
		t.Fatalf("ws send: %v", err)
	}

	var reply string
	if err := websocket.Message.Receive(ws, &reply); err != nil {
		t.Fatalf("ws receive: %v", err)
	}

	if reply != "echo: hello" {
		t.Errorf("got %q, want %q", reply, "echo: hello")
	}
}
