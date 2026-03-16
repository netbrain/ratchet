package events

import "time"

// EventType represents the kind of file-system event observed.
type EventType string

const (
	FileCreated  EventType = "file:created"
	FileModified EventType = "file:modified"
	FileDeleted  EventType = "file:deleted"
)

// Event is the canonical event emitted when a watched file changes.
// The optional Data field carries a type-specific payload when the event
// has been enriched by the pipeline (e.g., parsed DebateMeta).
type Event struct {
	ID        uint64    `json:"id"`
	Type      EventType `json:"type"`
	Path      string    `json:"path"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}
