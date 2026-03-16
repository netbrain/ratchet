package sse

import (
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/events"
)

func TestBroker_SubscribeFrom_ReceivesMissedEvents(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(100)

	now := time.Now()
	// Publish 5 events.
	for i := uint64(1); i <= 5; i++ {
		b.Publish(events.Event{
			ID:        i,
			Type:      events.FileModified,
			Path:      "test.yaml",
			Timestamp: now,
		})
	}

	// Subscribe from ID 3 — should receive events 4 and 5.
	sub, err := b.SubscribeFrom(3)
	if err != nil {
		t.Fatalf("SubscribeFrom returned error: %v", err)
	}
	defer sub.Unsubscribe()

	var received []uint64
	timeout := time.After(time.Second)
	for range 2 {
		select {
		case ev := <-sub.Events():
			received = append(received, ev.ID)
		case <-timeout:
			t.Fatalf("timed out after receiving %d events", len(received))
		}
	}

	if len(received) != 2 {
		t.Fatalf("got %d events, want 2", len(received))
	}
	if received[0] != 4 {
		t.Errorf("first event ID: got %d, want 4", received[0])
	}
	if received[1] != 5 {
		t.Errorf("second event ID: got %d, want 5", received[1])
	}
}

func TestBroker_SubscribeFrom_BufferOverflow(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(5)

	now := time.Now()
	// Publish 10 events so the buffer only retains the last 5.
	for i := uint64(1); i <= 10; i++ {
		b.Publish(events.Event{
			ID:        i,
			Type:      events.FileModified,
			Path:      "test.yaml",
			Timestamp: now,
		})
	}

	// Request from ID 2 — which has been evicted.
	_, err := b.SubscribeFrom(2)
	if err == nil {
		t.Fatal("SubscribeFrom should return error when ID is no longer in buffer")
	}

	var idErr *ErrIDNotInBuffer
	ok := false
	if e, isType := err.(*ErrIDNotInBuffer); isType {
		idErr = e
		ok = true
	}
	if !ok {
		t.Fatalf("expected *ErrIDNotInBuffer, got %T: %v", err, err)
	}
	if idErr.RequestedID != 2 {
		t.Errorf("RequestedID: got %d, want 2", idErr.RequestedID)
	}
}

func TestBroker_SubscribeFrom_ZeroID_GetsAllBuffered(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(100)

	now := time.Now()
	for i := uint64(1); i <= 3; i++ {
		b.Publish(events.Event{
			ID:        i,
			Type:      events.FileCreated,
			Path:      "test.yaml",
			Timestamp: now,
		})
	}

	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatalf("SubscribeFrom(0) returned error: %v", err)
	}
	defer sub.Unsubscribe()

	var received []uint64
	timeout := time.After(time.Second)
	for range 3 {
		select {
		case ev := <-sub.Events():
			received = append(received, ev.ID)
		case <-timeout:
			t.Fatalf("timed out after receiving %d events", len(received))
		}
	}

	if len(received) != 3 {
		t.Fatalf("got %d events, want 3", len(received))
	}
}

func TestBroker_SubscribeFrom_ThenReceivesLive(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(100)

	now := time.Now()
	// Publish 2 buffered events.
	for i := uint64(1); i <= 2; i++ {
		b.Publish(events.Event{
			ID:        i,
			Type:      events.FileModified,
			Path:      "test.yaml",
			Timestamp: now,
		})
	}

	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatalf("SubscribeFrom returned error: %v", err)
	}
	defer sub.Unsubscribe()

	// Drain buffered events.
	for range 2 {
		select {
		case <-sub.Events():
		case <-time.After(time.Second):
			t.Fatal("timed out draining buffered events")
		}
	}

	// Now publish a live event.
	b.Publish(events.Event{
		ID:        3,
		Type:      events.FileCreated,
		Path:      "live.yaml",
		Timestamp: now,
	})

	select {
	case ev := <-sub.Events():
		if ev.ID != 3 {
			t.Errorf("live event ID: got %d, want 3", ev.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live event")
	}
}

func TestBroker_SetBufferSize(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	// Should not panic.
	b.SetBufferSize(50)
	b.SetBufferSize(200)
}
