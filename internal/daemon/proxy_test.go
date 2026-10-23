package daemon

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

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
	routes.update(map[string]route{"myapp.test": {Port: port}})

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
	routes.update(map[string]route{"myapp.test": {Port: port}})

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
	routes.update(map[string]route{})

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
	routes.update(map[string]route{"myapp.test": {Port: 59999}})

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
	routes.update(map[string]route{"myapp.test": {Port: port}})

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
	routes.update(map[string]route{})

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
	routes.update(map[string]route{"myapp.test": {Port: port}})

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

func TestProxyRoutesWildcardSubdomain(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tenant backend"))
	}))
	defer backend.Close()

	port := backendPort(t, backend)
	rt := &RouteTable{}
	rt.updateWithWildcards(
		map[string]route{"realty120.test": {Port: port}},
		map[string]route{"realty120.test": {Port: port}},
	)

	proxy := NewProxy(rt)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "rp.realty120.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "tenant backend" {
		t.Errorf("got %q, want %q", body, "tenant backend")
	}
}

func TestProxyExactMatchWinsOverWildcard(t *testing.T) {
	wildcard := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("wildcard"))
	}))
	defer wildcard.Close()

	exact := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("exact"))
	}))
	defer exact.Close()

	wildcardPort := backendPort(t, wildcard)
	exactPort := backendPort(t, exact)
	rt := &RouteTable{}
	rt.updateWithWildcards(
		map[string]route{
			"realty120.test":     {Port: wildcardPort},
			"api.realty120.test": {Port: exactPort},
		},
		map[string]route{"realty120.test": {Port: wildcardPort}},
	)

	proxy := NewProxy(rt)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "api.realty120.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "exact" {
		t.Errorf("got %q, want %q", body, "exact")
	}
}

func TestProxyHostOverrideRewritesHostHeader(t *testing.T) {
	var gotHost, gotFwdHost, gotFwdProto string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		gotFwdHost = r.Header.Get("X-Forwarded-Host")
		gotFwdProto = r.Header.Get("X-Forwarded-Proto")
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()

	port := backendPort(t, backend)
	routes := &RouteTable{}
	routes.update(map[string]route{
		"abc123.trycloudflare.com": {Port: port, HostOverride: "myapp.test"},
	})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "abc123.trycloudflare.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if gotHost != "myapp.test" {
		t.Errorf("backend saw Host %q, want %q", gotHost, "myapp.test")
	}
	if gotFwdHost != "abc123.trycloudflare.com" {
		t.Errorf("backend saw X-Forwarded-Host %q, want %q", gotFwdHost, "abc123.trycloudflare.com")
	}
	if gotFwdProto != "https" {
		t.Errorf("backend saw X-Forwarded-Proto %q, want %q", gotFwdProto, "https")
	}
}

func TestProxyCapsSimultaneousBackendConnections(t *testing.T) {
	var mu sync.Mutex
	open, peak := 0, 0
	backend := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte("ok"))
	}))
	backend.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		mu.Lock()
		defer mu.Unlock()
		switch state {
		case http.StateNew:
			open++
			peak = max(peak, open)
		case http.StateClosed, http.StateHijacked:
			open--
		}
	}
	backend.Start()
	defer backend.Close()

	routes := &RouteTable{}
	routes.update(map[string]route{"myapp.test": {Port: backendPort(t, backend)}})
	proxy := NewProxy(routes)

	// A browser loading a dev server's module graph sends hundreds of
	// requests at once over one HTTP/2 connection, so they reach the
	// handler together.
	const burst = 300
	var wg sync.WaitGroup
	statuses := make(chan int, burst)
	for range burst {
		wg.Go(func() {
			req, _ := http.NewRequest("GET", "http://myapp.test/", nil)
			rec := &statusRecorder{header: http.Header{}}
			proxy.ServeHTTP(rec, req)
			statuses <- rec.status
		})
	}
	wg.Wait()
	close(statuses)

	for status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("status: got %d, want 200", status)
		}
	}
	if peak > maxBackendConns {
		t.Errorf("backend saw %d simultaneous connections, want at most %d", peak, maxBackendConns)
	}
}

func TestProxyOpenWebSocketsLeaveConnectionsFree(t *testing.T) {
	// Every open tab of a dev server holds a hot-reload WebSocket for as
	// long as it stays open.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		websocket.Handler(func(ws *websocket.Conn) {
			var msg string
			_ = websocket.Message.Receive(ws, &msg)
		}).ServeHTTP(w, r)
	}))
	defer backend.Close()

	routes := &RouteTable{}
	routes.update(map[string]route{"myapp.test": {Port: backendPort(t, backend)}})
	srv := httptest.NewServer(NewProxy(routes))
	defer srv.Close()
	proxyURL, _ := url.Parse(srv.URL)

	for range maxBackendConns + 1 {
		config, _ := websocket.NewConfig("ws://myapp.test/", "http://myapp.test/")
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
	}

	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "myapp.test"
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request with %d WebSockets open: %v", maxBackendConns+1, err)
	}
	resp.Body.Close()
}

// statusRecorder is a minimal concurrent-safe http.ResponseWriter that keeps
// only the status code.
type statusRecorder struct {
	header http.Header
	status int
}

func (r *statusRecorder) Header() http.Header { return r.header }

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return io.Discard.Write(b)
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

// captureLog redirects the standard logger to a buffer for the rest of the
// test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

// closedPort returns a localhost port with nothing listening on it.
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestProxyLogsBackendErrorOnOneLine(t *testing.T) {
	logs := captureLog(t)
	port := closedPort(t)
	routes := &RouteTable{}
	routes.update(map[string]route{"myapp.test": {Port: port}})
	proxy := NewProxy(routes)

	// The path is whatever the client sent; an encoded newline must not
	// start a second, forged log line.
	req := httptest.NewRequest("GET", "http://myapp.test/a%0Aproxy%20error:%20forged", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status: got %d, want 502", rec.Code)
	}
	out := logs.String()
	if strings.Count(out, "\n") != 1 {
		t.Errorf("want exactly one log line, got %q", out)
	}
	if !strings.Contains(out, "port "+strconv.Itoa(port)) {
		t.Errorf("log should name port %d, got %q", port, out)
	}
}

func TestProxyDoesNotLogCancelledRequests(t *testing.T) {
	logs := captureLog(t)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()
	routes := &RouteTable{}
	routes.update(map[string]route{"myapp.test": {Port: backendPort(t, backend)}})
	proxy := NewProxy(routes)

	// A browser cancels every pending request when the user reloads or
	// navigates away; those are not backend failures.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("GET", "http://myapp.test/", nil).WithContext(ctx)
	proxy.ServeHTTP(httptest.NewRecorder(), req)

	if out := logs.String(); out != "" {
		t.Errorf("cancelled request should not be logged, got %q", out)
	}
}
