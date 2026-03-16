package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDomainEventType_Values(t *testing.T) {
	tests := []struct {
		name     string
		et       EventType
		expected string
	}{
		{"DebateStarted", DebateStarted, "debate:started"},
		{"DebateUpdated", DebateUpdated, "debate:updated"},
		{"DebateResolved", DebateResolved, "debate:resolved"},
		{"ScoreUpdated", ScoreUpdated, "score:updated"},
		{"PairModified", PairModified, "pair:modified"},
		{"PlanUpdated", PlanUpdated, "plan:updated"},
		{"ConfigChanged", ConfigChanged, "config:changed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.et) != tt.expected {
				t.Errorf("got %q, want %q", tt.et, tt.expected)
			}
		})
	}
}

func TestEvent_WithData_JSONRoundTrip(t *testing.T) {
	ts := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		event Event
	}{
		{
			name: "debate started with nil data",
			event: Event{
				ID:        1,
				Type:      DebateStarted,
				Path:      "debates/foo/meta.json",
				Timestamp: ts,
				Data:      nil,
			},
		},
		{
			name: "score updated with map data",
			event: Event{
				ID:        2,
				Type:      ScoreUpdated,
				Path:      "scores/scores.jsonl",
				Timestamp: ts,
				Data:      map[string]any{"pair": "api-design", "milestone": float64(2)},
			},
		},
		{
			name: "plan updated",
			event: Event{
				ID:        3,
				Type:      PlanUpdated,
				Path:      "plan.yaml",
				Timestamp: ts,
				Data:      nil,
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

func TestEvent_WithData_JSONFields(t *testing.T) {
	ts := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	ev := Event{
		ID:        7,
		Type:      DebateUpdated,
		Path:      "debates/foo/meta.json",
		Timestamp: ts,
		Data:      map[string]string{"status": "in_progress"},
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	expectedKeys := []string{"id", "type", "path", "timestamp", "data"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing expected JSON key %q", key)
		}
	}
}

func TestEvent_OmitsNilData(t *testing.T) {
	ev := Event{
		ID:        1,
		Type:      PlanUpdated,
		Path:      "plan.yaml",
		Timestamp: time.Now(),
		Data:      nil,
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if _, ok := raw["data"]; ok {
		t.Error("data field should be omitted when nil (omitempty)")
	}
}
