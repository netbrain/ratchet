package sse

import (
	"github.com/netbrain/ratchet-monitor/internal/events"
)

// Uses events.Event via the Broker's existing Publish method and Subscription type.

// ErrIDNotInBuffer indicates the requested event ID is no longer in the ring
// buffer, meaning the client must perform a full resync.
type ErrIDNotInBuffer struct {
	RequestedID uint64
	OldestID    uint64
}

func (e *ErrIDNotInBuffer) Error() string {
	return "requested event ID is no longer in buffer; resync required"
}

// SubscribeFrom creates a subscription that replays events starting after
// lastEventID from the ring buffer, then continues with live events.
// Returns ErrIDNotInBuffer if lastEventID has been evicted from the buffer.
func (b *Broker) SubscribeFrom(lastEventID uint64) (*Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrBrokerClosed
	}

	// Determine which buffered events to replay.
	var replay []events.Event

	if len(b.buffer) > 0 && lastEventID > 0 {
		oldestID := b.buffer[0].ID

		// Check if the requested ID has been evicted.
		if lastEventID < oldestID {
			return nil, &ErrIDNotInBuffer{
				RequestedID: lastEventID,
				OldestID:    oldestID,
			}
		}

		// Find events after lastEventID.
		for _, ev := range b.buffer {
			if ev.ID > lastEventID {
				replay = append(replay, ev)
			}
		}
	} else if lastEventID == 0 {
		// Replay all buffered events.
		replay = make([]events.Event, len(b.buffer))
		copy(replay, b.buffer)
	}

	// Create the subscription with enough buffer for replay + live events.
	// This ensures the replay send never blocks and can't panic if the
	// subscription is unsubscribed before replay completes.
	sub := &Subscription{
		ch:     make(chan events.Event, 64+len(replay)),
		broker: b,
	}
	b.subscribers[sub] = struct{}{}

	// Send replayed events directly into the buffered channel.
	// The channel is large enough to hold all replay events without blocking.
	for _, ev := range replay {
		sub.ch <- ev
	}

	return sub, nil
}

// SetBufferSize configures the ring buffer capacity.
// If the buffer already contains events and the new size is smaller,
// the oldest events are discarded to fit the new capacity.
// A size of 0 disables buffering and discards all buffered events.
func (b *Broker) SetBufferSize(size int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if size < 0 {
		size = 0
	}

	b.bufferSize = size

	if size == 0 {
		b.buffer = nil
		return
	}

	if b.buffer == nil {
		b.buffer = make([]events.Event, 0, size)
		return
	}

	// Trim if existing buffer exceeds new capacity.
	if len(b.buffer) > size {
		trimmed := make([]events.Event, size)
		copy(trimmed, b.buffer[len(b.buffer)-size:])
		b.buffer = trimmed
	}
}
