package events

import (
	"encoding/json"
	"testing"
	"time"
)

// TestEvent_V2Fields verifies that v2 fields (workspace, issue) are correctly
// serialized to JSON and can be omitted when empty.
func TestEvent_V2Fields(t *testing.T) {
	tests := []struct {
		name      string
		event     Event
		wantJSON  string
		checkKeys []string
	}{
		{
			name: "workspace and issue present",
			event: Event{
				ID:        1,
				Type:      FileModified,
				Path:      "/test/path",
				Timestamp: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
				Workspace: "monitor",
				Issue:     "issue-2-3",
			},
			checkKeys: []string{"workspace", "issue"},
		},
		{
			name: "workspace only",
			event: Event{
				ID:        2,
				Type:      FileCreated,
				Path:      "/test/path",
				Timestamp: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
				Workspace: "api",
			},
			checkKeys: []string{"workspace"},
		},
		{
			name: "no v2 fields (backward compatible)",
			event: Event{
				ID:        3,
				Type:      FileDeleted,
				Path:      "/test/path",
				Timestamp: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
			},
			checkKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("failed to marshal event: %v", err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("failed to unmarshal JSON: %v", err)
			}

			// Check that expected keys are present
			for _, key := range tt.checkKeys {
				if _, ok := decoded[key]; !ok {
					t.Errorf("expected key %q in JSON, got: %s", key, string(data))
				}
			}

			// Verify workspace field
			if tt.event.Workspace != "" {
				if ws, ok := decoded["workspace"].(string); !ok || ws != tt.event.Workspace {
					t.Errorf("workspace: got %v, want %q", decoded["workspace"], tt.event.Workspace)
				}
			} else {
				// Empty workspace should be omitted due to omitempty
				if _, ok := decoded["workspace"]; ok {
					t.Errorf("expected workspace to be omitted, got: %v", decoded["workspace"])
				}
			}

			// Verify issue field
			if tt.event.Issue != "" {
				if issue, ok := decoded["issue"].(string); !ok || issue != tt.event.Issue {
					t.Errorf("issue: got %v, want %q", decoded["issue"], tt.event.Issue)
				}
			} else {
				// Empty issue should be omitted due to omitempty
				if _, ok := decoded["issue"]; ok {
					t.Errorf("expected issue to be omitted, got: %v", decoded["issue"])
				}
			}

			// Verify roundtrip
			var roundtrip Event
			if err := json.Unmarshal(data, &roundtrip); err != nil {
				t.Fatalf("failed to unmarshal roundtrip: %v", err)
			}

			if roundtrip.Workspace != tt.event.Workspace {
				t.Errorf("roundtrip workspace: got %q, want %q", roundtrip.Workspace, tt.event.Workspace)
			}
			if roundtrip.Issue != tt.event.Issue {
				t.Errorf("roundtrip issue: got %q, want %q", roundtrip.Issue, tt.event.Issue)
			}
		})
	}
}
