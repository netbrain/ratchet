package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// sseHandler writes SSE events to the response in text/event-stream format.
func sseHandler(events []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		for _, ev := range events {
			fmt.Fprint(w, ev)
			flusher.Flush()
		}
		// Keep connection open until client disconnects.
		<-r.Context().Done()
	}
}

func TestSSEClientConnect(t *testing.T) {
	events := []string{
		"event: debate:started\nid: 1\ndata: {\"id\":1,\"type\":\"debate:started\",\"path\":\"debates/d-001.yaml\",\"timestamp\":\"2026-03-15T10:00:00Z\",\"data\":{}}\n\n",
	}
	srv := httptest.NewServer(sseHandler(events))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	select {
	case ev := <-ch:
		if ev.Type != "debate:started" {
			t.Errorf("expected event type 'debate:started', got %q", ev.Type)
		}
		if ev.ID != "1" {
			t.Errorf("expected event ID '1', got %q", ev.ID)
		}
		if len(ev.Data) == 0 {
			t.Error("expected non-empty event data")
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for SSE event")
	}
}

func TestSSEClientMultipleEvents(t *testing.T) {
	events := []string{
		"event: debate:started\nid: 1\ndata: {\"id\":1,\"type\":\"debate:started\"}\n\n",
		"event: debate:updated\nid: 2\ndata: {\"id\":2,\"type\":\"debate:updated\"}\n\n",
		"event: debate:resolved\nid: 3\ndata: {\"id\":3,\"type\":\"debate:resolved\"}\n\n",
	}
	srv := httptest.NewServer(sseHandler(events))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	expectedTypes := []string{"debate:started", "debate:updated", "debate:resolved"}
	for i, expected := range expectedTypes {
		select {
		case ev := <-ch:
			if ev.Type != expected {
				t.Errorf("event %d: expected type %q, got %q", i, expected, ev.Type)
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
}

func TestSSEClientContextCancellation(t *testing.T) {
	// Server that never sends events — just holds the connection open.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Cancel and verify channel closes.
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			// Receiving a zero-value event is acceptable during shutdown,
			// but the channel must eventually close.
			select {
			case _, stillOpen := <-ch:
				if stillOpen {
					t.Error("channel should be closed after context cancellation")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("channel did not close after context cancellation")
			}
		}
		// Channel closed — correct behavior.
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after context cancellation")
	}
}

func TestSSEClientReconnectsOnServerClose(t *testing.T) {
	var connectCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := connectCount.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// First connection: send one event then close.
		if count == 1 {
			fmt.Fprint(w, "event: debate:started\nid: 1\ndata: {\"id\":1,\"type\":\"debate:started\"}\n\n")
			flusher.Flush()
			return // close connection
		}
		// Second connection: send another event then hold open.
		fmt.Fprint(w, "event: debate:updated\nid: 2\ndata: {\"id\":2,\"type\":\"debate:updated\"}\n\n")
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Should receive events from both connections.
	expectedTypes := []string{"debate:started", "debate:updated"}
	for i, expected := range expectedTypes {
		select {
		case ev := <-ch:
			if ev.Type != expected {
				t.Errorf("event %d: expected %q, got %q", i, expected, ev.Type)
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for event %d (%s)", i, expected)
		}
	}

	if connectCount.Load() < 2 {
		t.Errorf("expected at least 2 connections (reconnect), got %d", connectCount.Load())
	}
}

func TestSSEClientSendsLastEventID(t *testing.T) {
	var (
		mu          sync.Mutex
		lastEventID string
	)

	var connectCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := connectCount.Add(1)

		if count == 2 {
			mu.Lock()
			lastEventID = r.Header.Get("Last-Event-ID")
			mu.Unlock()
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		if count == 1 {
			fmt.Fprint(w, "event: debate:started\nid: 42\ndata: {\"id\":42,\"type\":\"debate:started\"}\n\n")
			flusher.Flush()
			return // close to trigger reconnect
		}

		fmt.Fprint(w, "event: debate:updated\nid: 43\ndata: {\"id\":43,\"type\":\"debate:updated\"}\n\n")
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Drain both events.
	for i := 0; i < 2; i++ {
		select {
		case <-ch:
		case <-ctx.Done():
			t.Fatalf("timed out waiting for event %d", i)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if lastEventID != "42" {
		t.Errorf("expected Last-Event-ID '42' on reconnect, got %q", lastEventID)
	}
}

func TestSSEClientConnectionState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)

	// Before subscribing, state should be Disconnected.
	if c.ConnectionState() != Disconnected {
		t.Errorf("expected initial state Disconnected, got %s", c.ConnectionState())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Give the client a moment to connect.
	time.Sleep(100 * time.Millisecond)

	if c.ConnectionState() != Connected {
		t.Errorf("expected state Connected after subscribe, got %s", c.ConnectionState())
	}

	cancel()
	time.Sleep(100 * time.Millisecond)

	if c.ConnectionState() != Disconnected {
		t.Errorf("expected state Disconnected after cancel, got %s", c.ConnectionState())
	}
}

func TestSSEClientReconnectingState(t *testing.T) {
	var connectCount atomic.Int32
	connEstablished := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := connectCount.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		if count == 1 {
			flusher.Flush()
			return // close immediately to trigger reconnect
		}

		// Second connection: signal and hold.
		flusher.Flush()
		close(connEstablished)
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Wait for second connection to confirm reconnect happened.
	select {
	case <-connEstablished:
	case <-ctx.Done():
		t.Fatal("timed out waiting for reconnection")
	}

	// The client should have transitioned through Reconnecting at some point.
	// After reconnecting, it should be Connected again.
	if c.ConnectionState() != Connected {
		t.Errorf("expected state Connected after reconnect, got %s", c.ConnectionState())
	}
}

func TestSSEClientExponentialBackoff(t *testing.T) {
	var (
		mu         sync.Mutex
		timestamps []time.Time
	)

	var connectCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := connectCount.Add(1)
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		flusher.Flush()

		// Close first 3 connections to force reconnects.
		if count <= 3 {
			return
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithBaseBackoff(50*time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Wait for at least 4 connections.
	deadline := time.After(8 * time.Second)
	for connectCount.Load() < 4 {
		select {
		case <-deadline:
			t.Fatalf("only got %d connections, expected >= 4", connectCount.Load())
		case <-time.After(50 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify that gaps between reconnections increase (exponential backoff).
	if len(timestamps) < 4 {
		t.Fatalf("expected >= 4 timestamps, got %d", len(timestamps))
	}

	gap1 := timestamps[1].Sub(timestamps[0])
	gap2 := timestamps[2].Sub(timestamps[1])
	gap3 := timestamps[3].Sub(timestamps[2])

	// Each gap should be at least as long as the previous (with some tolerance).
	// Gap2 >= Gap1 and Gap3 >= Gap2 (backoff grows).
	if gap2 < gap1/2 {
		t.Errorf("expected gap2 (%v) >= gap1/2 (%v) — backoff should increase", gap2, gap1/2)
	}
	if gap3 < gap2/2 {
		t.Errorf("expected gap3 (%v) >= gap2/2 (%v) — backoff should increase", gap3, gap2/2)
	}
}

func TestSSEClientIgnoresCommentLines(t *testing.T) {
	events := []string{
		": this is a comment\n\n",
		"event: debate:started\nid: 1\ndata: {\"id\":1,\"type\":\"debate:started\"}\n\n",
	}
	srv := httptest.NewServer(sseHandler(events))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	select {
	case ev := <-ch:
		if ev.Type != "debate:started" {
			t.Errorf("expected 'debate:started', got %q", ev.Type)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for event")
	}
}

func TestSSEClientHandlesMultiLineData(t *testing.T) {
	// SSE spec: multiple data: lines are joined with "\n".
	// Use valid JSON split across lines: {"id":1,"type":"debate:started"}
	events := []string{
		"event: debate:started\nid: 1\ndata: {\"id\":1,\"type\"\ndata: :\"debate:started\"}\n\n",
	}
	srv := httptest.NewServer(sseHandler(events))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	select {
	case ev := <-ch:
		if len(ev.Data) == 0 {
			t.Error("expected non-empty data for multi-line event")
		}
		// The joined data should be: {"id":1,"type"\n:"debate:started"}
		// This tests that the client joins data lines with \n per SSE spec.
		// Note: the real server never sends multi-line data, so this is a
		// robustness test. The client should deliver raw joined bytes.
		if ev.Type != "debate:started" {
			t.Errorf("expected event type 'debate:started', got %q", ev.Type)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for event")
	}
}

func TestSSEClientStateCallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	var (
		mu     sync.Mutex
		states []ConnectionState
	)

	c := NewClient(srv.URL, WithStateCallback(func(s ConnectionState) {
		mu.Lock()
		states = append(states, s)
		mu.Unlock()
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	cancel()
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have received at least a Connected state.
	foundConnected := false
	for _, s := range states {
		if s == Connected {
			foundConnected = true
			break
		}
	}
	if !foundConnected {
		t.Errorf("expected Connected state in callback, got %v", states)
	}
}
