package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/netbrain/ratchet-monitor/internal/sse"
)

// SSEHandler returns a handler that streams events via Server-Sent Events.
// Each HTTP request gets its own broker subscription, ensuring independent
// client lifecycles.
func SSEHandler(broker *sse.Broker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		// Honor the Last-Event-ID header per the SSE specification.
		// If the client reconnects with a Last-Event-ID, replay missed events.
		var sub *sse.Subscription
		if lastID := r.Header.Get("Last-Event-ID"); lastID != "" {
			id, err := strconv.ParseUint(lastID, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid Last-Event-ID")
				return
			}
			sub, err = broker.SubscribeFrom(id)
			if err != nil {
				// Buffer overflow: client needs to do a full resync.
				writeError(w, http.StatusGone, err.Error())
				return
			}
		} else {
			var err error
			sub, err = broker.Subscribe()
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "event stream unavailable")
				return
			}
		}
		defer sub.Unsubscribe()

		setSecurityHeaders(w)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, _ := w.(http.Flusher)
		// Send an SSE comment to establish the stream. Without this,
		// browsers keep EventSource in "connecting" state until the
		// first real data arrives.
		_, _ = fmt.Fprint(w, ": ok\n\n")
		if flusher != nil {
			flusher.Flush()
		}

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-sub.Events():
				if !ok {
					return
				}
				data, err := json.Marshal(ev)
				if err != nil {
					slog.Error("failed to marshal event", "error", err)
					continue
				}
				_, _ = fmt.Fprintf(w, "event: %s\n", ev.Type)
				_, _ = fmt.Fprintf(w, "id: %d\n", ev.ID)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	})
}
