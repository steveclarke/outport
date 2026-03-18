package tunnel

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// mockProvider implements Provider for testing.
type mockProvider struct {
	name   string
	urls   map[int]string // port → URL
	delay  time.Duration
	failOn int // port that should fail
}

func (m *mockProvider) Name() string          { return m.name }
func (m *mockProvider) CheckAvailable() error { return nil }
func (m *mockProvider) Start(ctx context.Context, port int) (*Tunnel, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if port == m.failOn {
		return nil, fmt.Errorf("tunnel failed for port %d", port)
	}
	url, ok := m.urls[port]
	if !ok {
		return nil, fmt.Errorf("unexpected port %d", port)
	}
	return NewTunnel(url, port, func() error { return nil }), nil
}

// trackingMockProvider wraps mockProvider to track Stop() calls.
type trackingMockProvider struct {
	mockProvider
	onStop func()
}

func (m *trackingMockProvider) Start(ctx context.Context, port int) (*Tunnel, error) {
	tun, err := m.mockProvider.Start(ctx, port)
	if err != nil {
		return nil, err
	}
	// Wrap the stop function to track calls
	return NewTunnel(tun.URL, tun.Port, func() error { m.onStop(); return nil }), nil
}

func TestManager_StartAll_Success(t *testing.T) {
	p := &mockProvider{
		name: "mock",
		urls: map[int]string{
			3000: "https://aaa.example.com",
			5173: "https://bbb.example.com",
		},
	}
	m := NewManager(p, 5*time.Second)

	services := map[string]int{"web": 3000, "vite": 5173}
	tunnels, err := m.StartAll(context.Background(), services)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer m.StopAll()

	if len(tunnels) != 2 {
		t.Fatalf("got %d tunnels, want 2", len(tunnels))
	}

	byService := make(map[string]*Tunnel)
	for _, tun := range tunnels {
		byService[tun.Service] = tun
	}
	if byService["web"].URL != "https://aaa.example.com" {
		t.Errorf("web URL = %q, want %q", byService["web"].URL, "https://aaa.example.com")
	}
	if byService["vite"].URL != "https://bbb.example.com" {
		t.Errorf("vite URL = %q, want %q", byService["vite"].URL, "https://bbb.example.com")
	}
}

func TestManager_StartAll_OneFails(t *testing.T) {
	stopCount := atomic.Int32{}
	p := &trackingMockProvider{
		mockProvider: mockProvider{
			name:   "mock",
			urls:   map[int]string{3000: "https://aaa.example.com", 5173: "https://bbb.example.com"},
			failOn: 5173,
		},
		onStop: func() { stopCount.Add(1) },
	}

	m := NewManager(p, 5*time.Second)

	services := map[string]int{"web": 3000, "vite": 5173}
	_, err := m.StartAll(context.Background(), services)
	if err == nil {
		t.Fatal("expected error when one tunnel fails")
	}

	// Verify the successful tunnel was cleaned up (all-or-nothing)
	if got := stopCount.Load(); got != 1 {
		t.Errorf("Stop() called %d times, want 1 (all-or-nothing cleanup)", got)
	}
}

func TestManager_StartAll_Timeout(t *testing.T) {
	p := &mockProvider{
		name:  "mock",
		urls:  map[int]string{3000: "https://aaa.example.com"},
		delay: 10 * time.Second, // longer than timeout
	}
	m := NewManager(p, 100*time.Millisecond)

	services := map[string]int{"web": 3000}
	_, err := m.StartAll(context.Background(), services)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestManager_StartAll_EmptyServices(t *testing.T) {
	p := &mockProvider{name: "mock"}
	m := NewManager(p, 5*time.Second)

	tunnels, err := m.StartAll(context.Background(), map[string]int{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tunnels) != 0 {
		t.Errorf("got %d tunnels, want 0", len(tunnels))
	}
}
