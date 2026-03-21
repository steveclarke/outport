package dashboard

import (
	"testing"
	"time"
)

func TestBroadcasterSendToSubscriber(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	b.Send(Event{Type: "health", Data: `{"up":true}`})

	select {
	case evt := <-ch:
		if evt.Type != "health" {
			t.Errorf("type: got %q, want %q", evt.Type, "health")
		}
		if evt.Data != `{"up":true}` {
			t.Errorf("data: got %q, want %q", evt.Data, `{"up":true}`)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBroadcasterUnsubscribe(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()
	b.Unsubscribe(ch)

	if b.ClientCount() != 0 {
		t.Errorf("client count: got %d, want 0", b.ClientCount())
	}
}

func TestBroadcasterClientCount(t *testing.T) {
	b := NewBroadcaster()
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	if b.ClientCount() != 2 {
		t.Errorf("client count: got %d, want 2", b.ClientCount())
	}

	b.Unsubscribe(ch1)
	if b.ClientCount() != 1 {
		t.Errorf("client count after unsub: got %d, want 1", b.ClientCount())
	}

	b.Unsubscribe(ch2)
	if b.ClientCount() != 0 {
		t.Errorf("client count after both unsub: got %d, want 0", b.ClientCount())
	}
}

func TestBroadcasterSlowClientDoesNotBlock(t *testing.T) {
	b := NewBroadcaster()
	_ = b.Subscribe()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			b.Send(Event{Type: "health", Data: "test"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Send blocked on slow client")
	}
}
