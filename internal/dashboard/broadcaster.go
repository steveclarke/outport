package dashboard

import "sync"

type Event struct {
	Type string
	Data string
}

type Broadcaster struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[chan Event]struct{}),
	}
}

func (b *Broadcaster) Subscribe() chan Event {
	ch := make(chan Event, 16)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *Broadcaster) Send(evt Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (b *Broadcaster) ClientCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clients)
}
