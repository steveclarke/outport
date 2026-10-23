//go:build unix

package daemon

import (
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

// listenWithBacklog opens a TCP listener on 127.0.0.1 with the given listen
// queue length, which net.Listen does not expose.
func listenWithBacklog(t *testing.T, backlog int) net.Listener {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socket: %v", err)
	}
	if err := syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := syscall.Listen(fd, backlog); err != nil {
		t.Fatalf("listen: %v", err)
	}
	file := os.NewFile(uintptr(fd), "backlog-listener")
	defer file.Close()
	ln, err := net.FileListener(file)
	if err != nil {
		t.Fatalf("file listener: %v", err)
	}
	return ln
}

// slowAcceptListener models a busy dev server that takes a moment to accept
// each new connection, so a burst of connects piles up in the listen queue.
type slowAcceptListener struct {
	net.Listener
}

func (l slowAcceptListener) Accept() (net.Conn, error) {
	time.Sleep(2 * time.Millisecond)
	return l.Listener.Accept()
}

func TestProxyBurstToSlowBackendWithSmallListenQueue(t *testing.T) {
	ln := listenWithBacklog(t, 2*maxBackendConns)
	backend := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		_, _ = w.Write([]byte("ok"))
	})}
	go func() { _ = backend.Serve(slowAcceptListener{ln}) }()
	defer backend.Close()

	routes := &RouteTable{}
	routes.update(map[string]route{"myapp.test": {Port: ln.Addr().(*net.TCPAddr).Port}})
	proxy := NewProxy(routes)

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

	failed := 0
	for status := range statuses {
		if status != http.StatusOK {
			failed++
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d requests failed, want none", failed, burst)
	}
}
