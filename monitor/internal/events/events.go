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
//
// V2 additions:
// - Workspace: the workspace name (if running in multi-workspace mode)
// - Issue: the issue reference (if this event is associated with a specific issue)
type Event struct {
	ID        uint64    `json:"id"`
	Type      EventType `json:"type"`
	Path      string    `json:"path"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
	Workspace string    `json:"workspace,omitempty"` // v2: workspace context
	Issue     string    `json:"issue,omitempty"`     // v2: issue reference for debate events
}
