// Package classifier maps file paths and parsed content to domain events.
package classifier

import (
	"strings"

	"github.com/netbrain/ratchet-monitor/internal/events"
	"github.com/netbrain/ratchet-monitor/internal/parser"
)

// Classifier maps a file path and its parsed content into a DomainEvent type.
type Classifier struct{}

// New creates a new Classifier.
func New() *Classifier {
	return &Classifier{}
}

// Classify determines the appropriate DomainEvent type for the given file
// path and optional parsed content. If the path is not recognized, it
// falls back to the provided raw file event type.
func (c *Classifier) Classify(path string, content any, fallback events.EventType) events.EventType {
	// Normalize path separators.
	p := strings.ReplaceAll(path, "\\", "/")

	// debates/*/meta.json -> debate events based on status
	if strings.HasPrefix(p, "debates/") && strings.HasSuffix(p, "/meta.json") {
		if meta, ok := content.(*parser.DebateMeta); ok {
			switch meta.Status {
			case "initiated":
				return events.DebateStarted
			case "consensus", "escalated":
				return events.DebateResolved
			default:
				return events.DebateUpdated
			}
		}
		// If no parsed content but path matches, still classify as debate event.
		if fallback == events.FileCreated {
			return events.DebateStarted
		}
		return events.DebateUpdated
	}

	// debates/*/rounds/* -> debate updated
	if strings.HasPrefix(p, "debates/") && strings.Contains(p, "/rounds/") {
		return events.DebateUpdated
	}

	// scores/* -> score updated
	if strings.HasPrefix(p, "scores/") {
		return events.ScoreUpdated
	}

	// pairs/* -> pair modified
	if strings.HasPrefix(p, "pairs/") {
		return events.PairModified
	}

	// plan.yaml -> plan updated
	if p == "plan.yaml" || strings.HasSuffix(p, "/plan.yaml") {
		return events.PlanUpdated
	}

	// workflow.yaml, project.yaml -> config changed
	if p == "workflow.yaml" || strings.HasSuffix(p, "/workflow.yaml") ||
		p == "project.yaml" || strings.HasSuffix(p, "/project.yaml") {
		return events.ConfigChanged
	}

	return fallback
}
