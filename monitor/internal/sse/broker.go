package sse

import (
	"errors"
	"sync"

	"github.com/netbrain/ratchet-monitor/internal/events"
)

// ErrBrokerClosed is returned when attempting to subscribe to a closed broker.
var ErrBrokerClosed = errors.New("broker is closed")

// Subscription represents a client subscription to the event stream.
type Subscription struct {
	ch     chan events.Event
	broker *Broker
}

// Events returns the read-only channel of events for this subscription.
func (s *Subscription) Events() <-chan events.Event {
	return s.ch
}

// Unsubscribe removes this subscription from the broker.
func (s *Subscription) Unsubscribe() {
	s.broker.unsubscribe(s)
}

// Broker fans out published events to all active subscribers.
type Broker struct {
	mu          sync.RWMutex
	subscribers map[*Subscription]struct{}
	closed      bool
	buffer      []events.Event
	bufferSize  int
}

// NewBroker creates a new Broker ready to accept subscriptions.
func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[*Subscription]struct{}),
	}
}

// Subscribe creates a new Subscription. The returned Subscription
// receives all events published after the call to Subscribe.
// Returns ErrBrokerClosed if the broker has been shut down.
func (b *Broker) Subscribe() (*Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrBrokerClosed
	}

	sub := &Subscription{
		ch:     make(chan events.Event, 64),
		broker: b,
	}
	b.subscribers[sub] = struct{}{}
	return sub, nil
}

func (b *Broker) unsubscribe(s *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// If the broker is already closed, all channels have been closed.
	if b.closed {
		return
	}

	if _, ok := b.subscribers[s]; ok {
		delete(b.subscribers, s)
		close(s.ch)
	}
}

// SubscriberCount returns the number of active subscribers.
func (b *Broker) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// Publish sends an event to all current subscribers.
// It is a no-op if the broker has been closed.
func (b *Broker) Publish(e events.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	// Buffer the event if buffering is enabled.
	if b.bufferSize > 0 {
		if len(b.buffer) >= b.bufferSize {
			// Compact: copy tail into a new slice to release old backing array,
			// preventing a memory leak from the ever-growing underlying array.
			keep := b.bufferSize - 1 // make room for the new event
			fresh := make([]events.Event, keep, b.bufferSize)
			copy(fresh, b.buffer[len(b.buffer)-keep:])
			b.buffer = fresh
		}
		b.buffer = append(b.buffer, e)
	}

	for sub := range b.subscribers {
		select {
		case sub.ch <- e:
		default:
			// drop if subscriber is slow
		}
	}
}

// Close shuts down the broker and closes all subscriber channels.
func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true

	for sub := range b.subscribers {
		close(sub.ch)
		delete(b.subscribers, sub)
	}
}
