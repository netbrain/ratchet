package events

// Domain event type constants for .ratchet/ file changes.
const (
	DebateStarted  EventType = "debate:started"
	DebateUpdated  EventType = "debate:updated"
	DebateResolved EventType = "debate:resolved"
	ScoreUpdated   EventType = "score:updated"
	PairModified   EventType = "pair:modified"
	PlanUpdated    EventType = "plan:updated"
	ConfigChanged  EventType = "config:changed"
)
