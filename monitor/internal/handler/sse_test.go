package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/events"
	"github.com/netbrain/ratchet-monitor/internal/sse"
)

// readSSEEvent reads the next complete SSE event from the scanner.
// An SSE event is terminated by a blank line.
// Returns a map of field name -> value (e.g. "event" -> "file:modified").
func readSSEEvent(scanner *bufio.Scanner) map[string]string {
	fields := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(fields) > 0 {
				break // end of event
			}
			continue // skip blank lines between comments and events
		}
		// Skip SSE comments (lines starting with ':').
		if strings.HasPrefix(line, ":") {
			continue
		}
		if idx := strings.Index(line, ": "); idx > 0 {
			fields[line[:idx]] = line[idx+2:]
		} else if idx := strings.Index(line, ":"); idx > 0 {
			fields[line[:idx]] = line[idx+1:]
		}
	}
	return fields
}

// startSSEHandler starts the SSE handler in a goroutine and waits for the
// subscription to be active before returning. It publishes events after the
// handler has subscribed to avoid race conditions with per-request subscriptions.
func startSSEHandler(t *testing.T, broker *sse.Broker) (rec *httptest.ResponseRecorder, cancel context.CancelFunc, done chan struct{}) {
	t.Helper()
	h := SSEHandler(broker)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	rec = httptest.NewRecorder()

	done = make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	// Wait for the handler goroutine to subscribe. We poll the broker's
	// subscriber count to avoid a fixed sleep.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if broker.SubscriberCount() > 0 {
			return rec, cancel, done
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for handler to subscribe")
	return
}

func TestSSEHandler_ContentType(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	rec, cancel, done := startSSEHandler(t, broker)

	broker.Publish(events.Event{
		ID:   1,
		Type: events.FileModified, Path: "test.yaml",
		Timestamp: time.Now(),
	})

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type: got %q, want %q", ct, "text/event-stream")
	}
}

func TestSSEHandler_CacheControlDisabled(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	rec, cancel, done := startSSEHandler(t, broker)

	broker.Publish(events.Event{
		ID: 1, Type: events.FileModified, Path: "test.yaml", Timestamp: time.Now(),
	})

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control: got %q, want %q", cc, "no-cache")
	}
}

func TestSSEHandler_EventFormat(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	rec, cancel, done := startSSEHandler(t, broker)

	ts := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	ev := events.Event{
		ID:        7,
		Type:      events.FileModified,
		Path:      ".ratchet/plan.yaml",
		Timestamp: ts,
	}
	broker.Publish(ev)

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := rec.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))
	fields := readSSEEvent(scanner)

	if fields["event"] != string(events.FileModified) {
		t.Errorf("event field: got %q, want %q", fields["event"], events.FileModified)
	}

	if fields["id"] != "7" {
		t.Errorf("id field: got %q, want %q", fields["id"], "7")
	}

	dataStr, ok := fields["data"]
	if !ok {
		t.Fatal("missing 'data' field in SSE event")
	}

	var parsed events.Event
	if err := json.Unmarshal([]byte(dataStr), &parsed); err != nil {
		t.Fatalf("data is not valid JSON: %v\nraw: %s", err, dataStr)
	}

	if parsed.ID != ev.ID {
		t.Errorf("data.id: got %d, want %d", parsed.ID, ev.ID)
	}
	if parsed.Type != ev.Type {
		t.Errorf("data.type: got %q, want %q", parsed.Type, ev.Type)
	}
	if parsed.Path != ev.Path {
		t.Errorf("data.path: got %q, want %q", parsed.Path, ev.Path)
	}
}

func TestSSEHandler_MultipleEvents(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	rec, cancel, done := startSSEHandler(t, broker)

	now := time.Now()
	evts := []events.Event{
		{ID: 1, Type: events.FileCreated, Path: "a.yaml", Timestamp: now},
		{ID: 2, Type: events.FileModified, Path: "b.yaml", Timestamp: now},
		{ID: 3, Type: events.FileDeleted, Path: "c.yaml", Timestamp: now},
	}

	for _, ev := range evts {
		broker.Publish(ev)
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := rec.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))

	for i, want := range evts {
		fields := readSSEEvent(scanner)
		if len(fields) == 0 {
			t.Fatalf("event[%d]: no SSE event found in output", i)
		}

		if fields["event"] != string(want.Type) {
			t.Errorf("event[%d] type: got %q, want %q", i, fields["event"], want.Type)
		}

		var parsed events.Event
		if err := json.Unmarshal([]byte(fields["data"]), &parsed); err != nil {
			t.Fatalf("event[%d] data not valid JSON: %v", i, err)
		}
		if parsed.ID != want.ID {
			t.Errorf("event[%d] id: got %d, want %d", i, parsed.ID, want.ID)
		}
	}
}

func TestSSEHandler_MethodNotAllowed(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	h := SSEHandler(broker)
	req := httptest.NewRequest(http.MethodPost, "/events", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if allow := rec.Header().Get("Allow"); allow == "" {
		t.Error("missing Allow header on 405 response")
	}
}

func TestSSEHandler_LastEventID_Replay(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()
	broker.SetBufferSize(100)

	now := time.Now()
	for i := uint64(1); i <= 3; i++ {
		broker.Publish(events.Event{
			ID: i, Type: events.FileModified, Path: "test.yaml", Timestamp: now,
		})
	}

	h := SSEHandler(broker)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	req.Header.Set("Last-Event-ID", "1")
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	// Wait for subscription
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if broker.SubscriberCount() > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := rec.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))

	// Should receive events 2 and 3 (after lastEventID=1).
	var ids []string
	for {
		fields := readSSEEvent(scanner)
		if len(fields) == 0 {
			break
		}
		if id, ok := fields["id"]; ok {
			ids = append(ids, id)
		}
	}

	if len(ids) != 2 {
		t.Fatalf("expected 2 replayed events, got %d: %v", len(ids), ids)
	}
	if ids[0] != "2" || ids[1] != "3" {
		t.Errorf("expected event IDs [2, 3], got %v", ids)
	}
}

func TestSSEHandler_LastEventID_Invalid(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	h := SSEHandler(broker)
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	req.Header.Set("Last-Event-ID", "not-a-number")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSSEHandler_LastEventID_Evicted(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()
	broker.SetBufferSize(2)

	now := time.Now()
	for i := uint64(1); i <= 5; i++ {
		broker.Publish(events.Event{
			ID: i, Type: events.FileModified, Path: "test.yaml", Timestamp: now,
		})
	}

	h := SSEHandler(broker)
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	req.Header.Set("Last-Event-ID", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusGone)
	}
}

func TestSSEHandler_XContentTypeOptions(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	rec, cancel, done := startSSEHandler(t, broker)

	broker.Publish(events.Event{
		ID: 1, Type: events.FileModified, Path: "test.yaml", Timestamp: time.Now(),
	})

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	xcto := rec.Header().Get("X-Content-Type-Options")
	if xcto != "nosniff" {
		t.Errorf("X-Content-Type-Options: got %q, want %q", xcto, "nosniff")
	}
}

func TestSSEHandler_BrokerCloseTerminatesStream(t *testing.T) {
	broker := sse.NewBroker()

	rec, _, done := startSSEHandler(t, broker)

	broker.Publish(events.Event{
		ID: 1, Type: events.FileModified, Path: "test.yaml", Timestamp: time.Now(),
	})
	time.Sleep(50 * time.Millisecond)

	// Closing the broker should cause the handler to return
	// because sub.Events() channel will be closed.
	broker.Close()

	select {
	case <-done:
		// Handler exited cleanly -- this is the expected path.
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit after broker.Close()")
	}

	// Verify the event we published before close was still received.
	body := rec.Body.String()
	if !strings.Contains(body, "test.yaml") {
		t.Errorf("expected event in body before broker close; got: %q", body)
	}
}

func TestSSEHandler_IndependentSubscriptions(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	h := SSEHandler(broker)

	// Start two independent clients
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			ctx, cancel := context.WithCancel(context.Background())
			req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
			rec := httptest.NewRecorder()

			done := make(chan struct{})
			go func() {
				h.ServeHTTP(rec, req)
				close(done)
			}()

			// Wait for subscription
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if broker.SubscriberCount() > clientID {
					break
				}
				time.Sleep(time.Millisecond)
			}

			time.Sleep(20 * time.Millisecond)
			cancel()
			<-done
		}(i)
	}

	wg.Wait()

	// After both clients disconnect, a third client should still work
	rec, cancel, done := startSSEHandler(t, broker)
	broker.Publish(events.Event{
		ID: 99, Type: events.FileCreated, Path: "z.yaml", Timestamp: time.Now(),
	})
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := rec.Body.String()
	if !strings.Contains(body, "z.yaml") {
		t.Errorf("third client should receive events; got body: %q", body)
	}
}
