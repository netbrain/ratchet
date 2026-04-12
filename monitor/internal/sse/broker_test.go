package sse

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/events"
)

func TestNewBroker(t *testing.T) {
	b := NewBroker()
	if b == nil {
		t.Fatal("NewBroker returned nil")
	}
	b.Close()
}

func TestBroker_SubscribeReturnsNonNil(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	sub, err := b.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	if sub == nil {
		t.Fatal("Subscribe returned nil")
	}
	sub.Unsubscribe()
}

func TestBroker_PublishToSingleSubscriber(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	sub, err := b.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer sub.Unsubscribe()

	ev := events.Event{
		ID:        1,
		Type:      events.FileModified,
		Path:      ".ratchet/plan.yaml",
		Timestamp: time.Now(),
	}

	b.Publish(ev)

	select {
	case got := <-sub.Events():
		if got.ID != ev.ID {
			t.Errorf("ID: got %d, want %d", got.ID, ev.ID)
		}
		if got.Type != ev.Type {
			t.Errorf("Type: got %q, want %q", got.Type, ev.Type)
		}
		if got.Path != ev.Path {
			t.Errorf("Path: got %q, want %q", got.Path, ev.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBroker_FanOutToMultipleSubscribers(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	const numSubs = 5
	subs := make([]*Subscription, numSubs)
	for i := range subs {
		var err error
		subs[i], err = b.Subscribe()
		if err != nil {
			t.Fatalf("Subscribe[%d] returned error: %v", i, err)
		}
		defer subs[i].Unsubscribe()
	}

	ev := events.Event{
		ID:        42,
		Type:      events.FileCreated,
		Path:      ".ratchet/new.yaml",
		Timestamp: time.Now(),
	}

	b.Publish(ev)

	for i, sub := range subs {
		select {
		case got := <-sub.Events():
			if got.ID != ev.ID {
				t.Errorf("sub[%d] ID: got %d, want %d", i, got.ID, ev.ID)
			}
		case <-time.After(time.Second):
			t.Fatalf("sub[%d] timed out waiting for event", i)
		}
	}
}

func TestBroker_UnsubscribeStopsDelivery(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	sub, err := b.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	sub.Unsubscribe()

	ev := events.Event{
		ID:        99,
		Type:      events.FileDeleted,
		Path:      ".ratchet/gone.yaml",
		Timestamp: time.Now(),
	}

	b.Publish(ev)

	// Channel should be closed or not receive the event
	select {
	case _, ok := <-sub.Events():
		if ok {
			t.Error("received event after unsubscribe; expected channel closed or empty")
		}
	case <-time.After(100 * time.Millisecond):
		// Acceptable: no event delivered
	}
}

func TestBroker_CloseClosesAllSubscriberChannels(t *testing.T) {
	b := NewBroker()

	sub1, err := b.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	sub2, err := b.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	b.Close()

	// Both subscriber channels should be closed
	for i, sub := range []*Subscription{sub1, sub2} {
		select {
		case _, ok := <-sub.Events():
			if ok {
				t.Errorf("sub[%d]: channel still open after Close", i)
			}
		case <-time.After(time.Second):
			t.Errorf("sub[%d]: timed out; channel not closed after Close", i)
		}
	}
}

func TestBroker_PublishMultipleEventsOrdered(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	sub, err := b.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer sub.Unsubscribe()

	now := time.Now()
	evts := []events.Event{
		{ID: 1, Type: events.FileCreated, Path: "a.yaml", Timestamp: now},
		{ID: 2, Type: events.FileModified, Path: "b.yaml", Timestamp: now},
		{ID: 3, Type: events.FileDeleted, Path: "c.yaml", Timestamp: now},
	}

	for _, ev := range evts {
		b.Publish(ev)
	}

	for i, want := range evts {
		select {
		case got := <-sub.Events():
			if got.ID != want.ID {
				t.Errorf("event[%d] ID: got %d, want %d", i, got.ID, want.ID)
			}
		case <-time.After(time.Second):
			t.Fatalf("event[%d]: timed out", i)
		}
	}
}

// --- Edge-case and hardening tests ---

func TestBroker_PublishAfterClose(t *testing.T) {
	b := NewBroker()
	b.SetBufferSize(10)
	b.Close()

	// Must not panic.
	b.Publish(events.Event{ID: 1, Type: events.FileCreated, Path: "x"})
}

func TestBroker_DoubleClose(t *testing.T) {
	b := NewBroker()
	sub, _ := b.Subscribe()
	_ = sub

	// Must not panic on double close.
	b.Close()
	b.Close()
}

func TestBroker_DoubleUnsubscribe(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	sub, err := b.Subscribe()
	if err != nil {
		t.Fatal(err)
	}

	// Must not panic on double unsubscribe.
	sub.Unsubscribe()
	sub.Unsubscribe()
}

func TestBroker_UnsubscribeAfterClose(t *testing.T) {
	b := NewBroker()
	sub, err := b.Subscribe()
	if err != nil {
		t.Fatal(err)
	}

	b.Close()

	// Must not panic: unsubscribe after broker is closed.
	sub.Unsubscribe()
}

func TestBroker_SubscribeAfterClose(t *testing.T) {
	b := NewBroker()
	b.Close()

	_, err := b.Subscribe()
	if !errors.Is(err, ErrBrokerClosed) {
		t.Fatalf("expected ErrBrokerClosed, got %v", err)
	}
}

func TestBroker_SubscribeFromAfterClose(t *testing.T) {
	b := NewBroker()
	b.SetBufferSize(10)
	b.Close()

	_, err := b.SubscribeFrom(0)
	if !errors.Is(err, ErrBrokerClosed) {
		t.Fatalf("expected ErrBrokerClosed, got %v", err)
	}
}

func TestBroker_SlowConsumerDoesNotBlock(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	sub, _ := b.Subscribe()
	defer sub.Unsubscribe()

	// Fill the channel buffer (64) and then some.
	for i := uint64(0); i < 200; i++ {
		b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
	}

	// Should not have blocked. Drain what we can.
	count := 0
	for {
		select {
		case <-sub.Events():
			count++
		default:
			goto done
		}
	}
done:
	if count != 64 {
		t.Fatalf("expected 64 events (channel buffer size), got %d", count)
	}
}

func TestBroker_SubscribeFromZeroSizeBuffer(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	// bufferSize is 0 (default), no SetBufferSize called.

	// SubscribeFrom with lastEventID=0 should succeed with no replay.
	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatalf("SubscribeFrom(0) with zero buffer: %v", err)
	}
	defer sub.Unsubscribe()

	// No events should be pending.
	select {
	case ev := <-sub.Events():
		t.Fatalf("unexpected event: %v", ev)
	default:
	}
}

func TestBroker_SubscribeFromNonZeroIDZeroBuffer(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	// bufferSize is 0, no buffer.

	// SubscribeFrom with a non-zero lastEventID and no buffer.
	// Should succeed (no buffer to check), effectively a fresh subscribe.
	sub, err := b.SubscribeFrom(42)
	if err != nil {
		t.Fatalf("SubscribeFrom(42) with zero buffer: %v", err)
	}
	defer sub.Unsubscribe()
}

func TestBroker_SubscribeFromEvictedID(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(3)

	for i := uint64(1); i <= 5; i++ {
		b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
	}

	// ID 1 should have been evicted (buffer holds 3,4,5).
	_, err := b.SubscribeFrom(1)
	var notInBuf *ErrIDNotInBuffer
	if !errors.As(err, &notInBuf) {
		t.Fatalf("expected ErrIDNotInBuffer, got %v", err)
	}
	if notInBuf.RequestedID != 1 {
		t.Errorf("RequestedID: got %d, want 1", notInBuf.RequestedID)
	}
	if notInBuf.OldestID != 3 {
		t.Errorf("OldestID: got %d, want 3", notInBuf.OldestID)
	}
}

func TestBroker_SubscribeFromReplayAll(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(5)

	for i := uint64(1); i <= 3; i++ {
		b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
	}

	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	for i := uint64(1); i <= 3; i++ {
		select {
		case ev := <-sub.Events():
			if ev.ID != i {
				t.Errorf("replay[%d]: got ID %d", i, ev.ID)
			}
		case <-time.After(time.Second):
			t.Fatalf("replay[%d]: timed out", i)
		}
	}
}

func TestBroker_SubscribeFromPartialReplay(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(10)

	for i := uint64(1); i <= 5; i++ {
		b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
	}

	// Subscribe from ID 3: should replay events 4 and 5.
	sub, err := b.SubscribeFrom(3)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	for _, wantID := range []uint64{4, 5} {
		select {
		case ev := <-sub.Events():
			if ev.ID != wantID {
				t.Errorf("got ID %d, want %d", ev.ID, wantID)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for ID %d", wantID)
		}
	}

	// No more pending.
	select {
	case ev := <-sub.Events():
		t.Fatalf("unexpected extra event: %v", ev)
	default:
	}
}

func TestBroker_SubscribeFromThenLive(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(10)

	b.Publish(events.Event{ID: 1, Type: events.FileCreated, Path: "x"})

	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	// Drain replay.
	<-sub.Events()

	// Publish a live event.
	b.Publish(events.Event{ID: 2, Type: events.FileModified, Path: "y"})

	select {
	case ev := <-sub.Events():
		if ev.ID != 2 {
			t.Errorf("live event ID: got %d, want 2", ev.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live event")
	}
}

func TestBroker_BufferEvictsOldest(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(3)

	for i := uint64(1); i <= 10; i++ {
		b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
	}

	// Buffer should contain events 8, 9, 10.
	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	for _, wantID := range []uint64{8, 9, 10} {
		select {
		case ev := <-sub.Events():
			if ev.ID != wantID {
				t.Errorf("got ID %d, want %d", ev.ID, wantID)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for ID %d", wantID)
		}
	}
}

func TestBroker_SetBufferSizeShrink(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(10)

	for i := uint64(1); i <= 8; i++ {
		b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
	}

	// Shrink to 3: should keep events 6, 7, 8.
	b.SetBufferSize(3)

	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	for _, wantID := range []uint64{6, 7, 8} {
		select {
		case ev := <-sub.Events():
			if ev.ID != wantID {
				t.Errorf("got ID %d, want %d", ev.ID, wantID)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for ID %d", wantID)
		}
	}
}

func TestBroker_SetBufferSizeToZeroDisables(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(10)

	b.Publish(events.Event{ID: 1, Type: events.FileCreated, Path: "x"})

	b.SetBufferSize(0)

	// Buffer should be nil now; SubscribeFrom(0) returns empty replay.
	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	select {
	case ev := <-sub.Events():
		t.Fatalf("unexpected event after buffer disabled: %v", ev)
	default:
	}
}

func TestBroker_SetBufferSizeNegative(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	// Should not panic; treated as 0.
	b.SetBufferSize(-5)

	b.Publish(events.Event{ID: 1, Type: events.FileCreated, Path: "x"})

	// No buffer, so SubscribeFrom(0) has no replay.
	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	select {
	case ev := <-sub.Events():
		t.Fatalf("unexpected event: %v", ev)
	default:
	}
}

func TestBroker_ConcurrentPublishSubscribe(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(100)

	var wg sync.WaitGroup

	// Concurrent publishers.
	for p := 0; p < 5; p++ {
		wg.Add(1)
		go func(offset uint64) {
			defer wg.Done()
			for i := uint64(0); i < 100; i++ {
				b.Publish(events.Event{ID: offset + i, Type: events.FileCreated, Path: "x"})
			}
		}(uint64(p) * 1000)
	}

	// Concurrent subscribers and unsubscribers.
	for s := 0; s < 5; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				sub, err := b.Subscribe()
				if err != nil {
					return // broker closed
				}
				// Drain a few events.
				for j := 0; j < 3; j++ {
					select {
					case <-sub.Events():
					case <-time.After(10 * time.Millisecond):
					}
				}
				sub.Unsubscribe()
			}
		}()
	}

	// Concurrent SubscribeFrom.
	for s := 0; s < 5; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				sub, err := b.SubscribeFrom(0)
				if err != nil {
					continue
				}
				select {
				case <-sub.Events():
				case <-time.After(10 * time.Millisecond):
				}
				sub.Unsubscribe()
			}
		}()
	}

	wg.Wait()
}

func TestBroker_ConcurrentCloseAndPublish(t *testing.T) {
	b := NewBroker()
	b.SetBufferSize(10)

	sub, _ := b.Subscribe()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := uint64(0); i < 1000; i++ {
			b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(time.Millisecond)
		b.Close()
	}()

	wg.Wait()

	// Channel should be closed after broker close.
	// Drain any remaining events.
	for range sub.Events() {
	}
}

func TestBroker_ConcurrentDoubleUnsubscribe(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	sub, _ := b.Subscribe()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		sub.Unsubscribe()
	}()

	go func() {
		defer wg.Done()
		sub.Unsubscribe()
	}()

	wg.Wait()
}

func TestBroker_SubscriberCountAccuracy(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	if got := b.SubscriberCount(); got != 0 {
		t.Fatalf("initial count: got %d, want 0", got)
	}

	sub1, _ := b.Subscribe()
	sub2, _ := b.Subscribe()

	if got := b.SubscriberCount(); got != 2 {
		t.Fatalf("after 2 subscribes: got %d, want 2", got)
	}

	sub1.Unsubscribe()

	if got := b.SubscriberCount(); got != 1 {
		t.Fatalf("after 1 unsubscribe: got %d, want 1", got)
	}

	sub2.Unsubscribe()

	if got := b.SubscriberCount(); got != 0 {
		t.Fatalf("after all unsubscribes: got %d, want 0", got)
	}
}

func TestBroker_BufferSizeOneEdge(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(1)

	for i := uint64(1); i <= 5; i++ {
		b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
	}

	// Buffer should only contain event 5.
	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	select {
	case ev := <-sub.Events():
		if ev.ID != 5 {
			t.Errorf("got ID %d, want 5", ev.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	// No more events.
	select {
	case ev := <-sub.Events():
		t.Fatalf("unexpected: %v", ev)
	default:
	}
}

func TestBroker_SubscribeFromLastEventIDEqualsNewest(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(10)

	for i := uint64(1); i <= 5; i++ {
		b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
	}

	// lastEventID == newest buffered ID: no replay needed.
	sub, err := b.SubscribeFrom(5)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	select {
	case ev := <-sub.Events():
		t.Fatalf("unexpected replay event: %v", ev)
	default:
	}
}

func TestBroker_SubscribeFromBeyondNewest(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	b.SetBufferSize(10)

	for i := uint64(1); i <= 5; i++ {
		b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
	}

	// lastEventID beyond what's buffered: no replay, no error.
	sub, err := b.SubscribeFrom(100)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	select {
	case ev := <-sub.Events():
		t.Fatalf("unexpected replay event: %v", ev)
	default:
	}
}

// --- Functional option tests ---

func TestNewBroker_DefaultBufferSize(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	if got := b.BufferSize(); got != DefaultBufferSize {
		t.Errorf("default buffer size: got %d, want %d", got, DefaultBufferSize)
	}
}

func TestNewBroker_WithBufferSize(t *testing.T) {
	b := NewBroker(WithBufferSize(256))
	defer b.Close()

	if got := b.BufferSize(); got != 256 {
		t.Errorf("buffer size: got %d, want 256", got)
	}
}

func TestNewBroker_WithBufferSizeClampMin(t *testing.T) {
	b := NewBroker(WithBufferSize(1))
	defer b.Close()

	if got := b.BufferSize(); got != MinBufferSize {
		t.Errorf("buffer size: got %d, want %d (MinBufferSize)", got, MinBufferSize)
	}
}

func TestNewBroker_WithBufferSizeClampMax(t *testing.T) {
	b := NewBroker(WithBufferSize(999999))
	defer b.Close()

	if got := b.BufferSize(); got != MaxBufferSize {
		t.Errorf("buffer size: got %d, want %d (MaxBufferSize)", got, MaxBufferSize)
	}
}

func TestNewBroker_WithBufferSizeBoundaries(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"below min", MinBufferSize - 1, MinBufferSize},
		{"at min", MinBufferSize, MinBufferSize},
		{"mid range", 512, 512},
		{"at max", MaxBufferSize, MaxBufferSize},
		{"above max", MaxBufferSize + 1, MaxBufferSize},
		{"zero", 0, MinBufferSize},
		{"negative", -100, MinBufferSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBroker(WithBufferSize(tt.in))
			defer b.Close()
			if got := b.BufferSize(); got != tt.want {
				t.Errorf("WithBufferSize(%d): got %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestNewBroker_WithBufferSizeRingBehavior(t *testing.T) {
	// Use a small custom buffer and verify eviction works correctly.
	b := NewBroker(WithBufferSize(64))
	defer b.Close()

	// Publish more events than the buffer can hold.
	for i := uint64(1); i <= 100; i++ {
		b.Publish(events.Event{ID: i, Type: events.FileCreated, Path: "x"})
	}

	// Buffer should hold events 37..100 (last 64).
	sub, err := b.SubscribeFrom(0)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	first := true
	count := 0
	for {
		select {
		case ev := <-sub.Events():
			if first {
				if ev.ID != 37 {
					t.Errorf("first buffered event ID: got %d, want 37", ev.ID)
				}
				first = false
			}
			count++
		default:
			goto done
		}
	}
done:
	if count != 64 {
		t.Errorf("buffered event count: got %d, want 64", count)
	}
}

func TestNewBroker_BackwardCompatNoArgs(t *testing.T) {
	// Verify zero-arg NewBroker still works and produces a functional broker.
	b := NewBroker()
	defer b.Close()

	sub, err := b.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	ev := events.Event{ID: 1, Type: events.FileCreated, Path: "test"}
	b.Publish(ev)

	select {
	case got := <-sub.Events():
		if got.ID != 1 {
			t.Errorf("event ID: got %d, want 1", got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}
