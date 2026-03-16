package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventType_Values(t *testing.T) {
	tests := []struct {
		name     string
		et       EventType
		expected string
	}{
		{"FileCreated", FileCreated, "file:created"},
		{"FileModified", FileModified, "file:modified"},
		{"FileDeleted", FileDeleted, "file:deleted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.et) != tt.expected {
				t.Errorf("got %q, want %q", tt.et, tt.expected)
			}
		})
	}
}

func TestEvent_JSONRoundTrip(t *testing.T) {
	ts := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		event Event
	}{
		{
			name: "created event",
			event: Event{
				ID:        1,
				Type:      FileCreated,
				Path:      ".ratchet/plan.yaml",
				Timestamp: ts,
			},
		},
		{
			name: "modified event",
			event: Event{
				ID:        42,
				Type:      FileModified,
				Path:      ".ratchet/scores/test.yaml",
				Timestamp: ts,
			},
		},
		{
			name: "deleted event",
			event: Event{
				ID:        100,
				Type:      FileDeleted,
				Path:      ".ratchet/old.yaml",
				Timestamp: ts,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var got Event
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if got.ID != tt.event.ID {
				t.Errorf("ID: got %d, want %d", got.ID, tt.event.ID)
			}
			if got.Type != tt.event.Type {
				t.Errorf("Type: got %q, want %q", got.Type, tt.event.Type)
			}
			if got.Path != tt.event.Path {
				t.Errorf("Path: got %q, want %q", got.Path, tt.event.Path)
			}
			if !got.Timestamp.Equal(tt.event.Timestamp) {
				t.Errorf("Timestamp: got %v, want %v", got.Timestamp, tt.event.Timestamp)
			}
		})
	}
}

func TestEvent_JSONFields(t *testing.T) {
	ts := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	ev := Event{
		ID:        7,
		Type:      FileModified,
		Path:      ".ratchet/plan.yaml",
		Timestamp: ts,
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	// Verify expected JSON keys exist
	expectedKeys := []string{"id", "type", "path", "timestamp"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing expected JSON key %q", key)
		}
	}

	// Verify no extra keys (data is omitted when nil via omitempty)
	if len(raw) != len(expectedKeys) {
		t.Errorf("got %d keys, want %d", len(raw), len(expectedKeys))
	}
}
