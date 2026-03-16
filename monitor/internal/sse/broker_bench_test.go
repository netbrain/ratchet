package sse

import (
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/events"
)

// BenchmarkBrokerPublish publishes to 10 subscribers.
func BenchmarkBrokerPublish(b *testing.B) {
	broker := NewBroker()
	defer broker.Close()

	subs := make([]*Subscription, 10)
	for i := range subs {
		var err error
		subs[i], err = broker.Subscribe()
		if err != nil {
			b.Fatal(err)
		}
		defer subs[i].Unsubscribe()
	}

	ev := events.Event{
		ID:        1,
		Type:      events.FileModified,
		Path:      ".ratchet/plan.yaml",
		Timestamp: time.Now(),
	}

	// Drain subscribers in background goroutines to avoid channel-full drops.
	for _, sub := range subs {
		go func(s *Subscription) {
			for range s.Events() {
			}
		}(sub)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.ID = uint64(i)
		broker.Publish(ev)
	}
}

// BenchmarkBrokerPublishBuffered publishes with ring buffer active.
func BenchmarkBrokerPublishBuffered(b *testing.B) {
	broker := NewBroker()
	defer broker.Close()
	broker.SetBufferSize(100)

	subs := make([]*Subscription, 10)
	for i := range subs {
		var err error
		subs[i], err = broker.Subscribe()
		if err != nil {
			b.Fatal(err)
		}
		defer subs[i].Unsubscribe()
	}

	ev := events.Event{
		ID:        1,
		Type:      events.FileModified,
		Path:      ".ratchet/plan.yaml",
		Timestamp: time.Now(),
	}

	// Drain subscribers in background goroutines.
	for _, sub := range subs {
		go func(s *Subscription) {
			for range s.Events() {
			}
		}(sub)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.ID = uint64(i)
		broker.Publish(ev)
	}
}
