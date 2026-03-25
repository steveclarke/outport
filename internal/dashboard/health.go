package dashboard

import (
	"sync"
	"time"

	"github.com/steveclarke/outport/internal/portcheck"
)

type PortProvider func() []int

type HealthChecker struct {
	mu       sync.Mutex
	previous map[int]bool
	provider PortProvider
	interval time.Duration
	ticker   *time.Ticker
	stopCh   chan struct{}
	onChange func(changes map[int]bool)
	running  bool
}

func NewHealthChecker(provider PortProvider, interval time.Duration, onChange func(map[int]bool)) *HealthChecker {
	return &HealthChecker{
		previous: make(map[int]bool),
		provider: provider,
		interval: interval,
		onChange: onChange,
	}
}

func (h *HealthChecker) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return
	}
	h.running = true
	h.stopCh = make(chan struct{})
	h.ticker = time.NewTicker(h.interval)
	go h.run()
}

func (h *HealthChecker) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.running {
		return
	}
	h.running = false
	h.ticker.Stop()
	close(h.stopCh)
}

func (h *HealthChecker) run() {
	h.check()
	for {
		select {
		case <-h.stopCh:
			return
		case <-h.ticker.C:
			h.check()
		}
	}
}

func (h *HealthChecker) check() {
	ports := h.provider()
	if len(ports) == 0 {
		return
	}
	current := portcheck.CheckAll(ports)
	h.mu.Lock()
	changes := detectChanges(h.previous, current)
	h.previous = current
	h.mu.Unlock()
	if len(changes) > 0 && h.onChange != nil {
		h.onChange(changes)
	}
}

func (h *HealthChecker) CheckNow() {
	h.check()
}

func (h *HealthChecker) CurrentStatus() map[int]bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make(map[int]bool, len(h.previous))
	for k, v := range h.previous {
		result[k] = v
	}
	return result
}

func detectChanges(prev, curr map[int]bool) map[int]bool {
	changes := make(map[int]bool)
	for port, up := range curr {
		if prevUp, ok := prev[port]; !ok || prevUp != up {
			changes[port] = up
		}
	}
	for port := range prev {
		if _, ok := curr[port]; !ok {
			changes[port] = false
		}
	}
	return changes
}
