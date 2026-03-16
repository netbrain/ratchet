// Package pipeline connects the file watcher to the SSE broker, enriching
// raw file-system events with parsed content and domain-level event types.
package pipeline

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/netbrain/ratchet-monitor/internal/classifier"
	"github.com/netbrain/ratchet-monitor/internal/events"
	"github.com/netbrain/ratchet-monitor/internal/parser"
	"github.com/netbrain/ratchet-monitor/internal/sse"
	"github.com/netbrain/ratchet-monitor/internal/watcher"
)

// Pipeline reads watcher events, classifies them, parses the corresponding
// file, and publishes enriched events to the SSE broker.
type Pipeline struct {
	watcher    *watcher.Watcher
	broker     *sse.Broker
	rootDir    string
	classifier *classifier.Classifier
}

// New creates a Pipeline that reads from w, enriches events using files
// under rootDir, and publishes to b.
func New(w *watcher.Watcher, b *sse.Broker, rootDir string) *Pipeline {
	return &Pipeline{
		watcher:    w,
		broker:     b,
		rootDir:    rootDir,
		classifier: classifier.New(),
	}
}

// Run processes watcher events until ctx is cancelled or the watcher
// channel closes.
func (p *Pipeline) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-p.watcher.Events():
			if !ok {
				return
			}
			enriched := p.enrich(ev)
			p.broker.Publish(enriched)
		}
	}
}

// enrich parses the file referenced by the event and classifies it into
// a domain event type. On parse failure, the raw file event is returned
// as a fallback.
//
// V2 enhancement: extracts workspace and issue context from the file path
// and parsed data.
func (p *Pipeline) enrich(ev events.Event) events.Event {
	// Compute the relative path within the .ratchet directory for classification.
	relPath := ev.Path
	if p.rootDir != "" {
		if rel, err := filepath.Rel(p.rootDir, ev.Path); err == nil {
			relPath = rel
		}
	}
	// Normalize to forward slashes for the classifier.
	relPath = strings.ReplaceAll(relPath, "\\", "/")

	// Extract workspace from path if present.
	// Workspace paths contain "workspaces/<name>/.ratchet/" pattern.
	ev.Workspace = extractWorkspace(relPath)

	// Don't try to parse deleted files.
	if ev.Type == events.FileDeleted {
		ev.Type = p.classifier.Classify(relPath, nil, ev.Type)
		return ev
	}

	// Try to read and parse the file.
	data, err := os.ReadFile(ev.Path)
	if err != nil {
		slog.Debug("pipeline: failed to read file", "path", ev.Path, "error", err)
		ev.Type = p.classifier.Classify(relPath, nil, ev.Type)
		return ev
	}

	parsed := p.parseFile(relPath, data)
	ev.Type = p.classifier.Classify(relPath, parsed, ev.Type)
	ev.Data = parsed

	// Extract issue context from debate metadata (if applicable).
	if meta, ok := parsed.(*parser.DebateMeta); ok && meta != nil {
		// Debate IDs often include issue refs in v2 (format: pair-YYYYMMDDTHHMMSS-issue-ref)
		// For now, we leave this empty - will be populated when debates include issue field.
		ev.Issue = ""
	}

	return ev
}

// extractWorkspace attempts to extract workspace name from a file path.
// Returns empty string if not in a workspace-specific path.
// Example: "workspaces/monitor/.ratchet/plan.yaml" -> "monitor"
func extractWorkspace(path string) string {
	// Check if path contains workspaces/ prefix
	if !strings.Contains(path, "workspaces/") {
		return ""
	}

	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "workspaces" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// parseFile attempts to parse a file based on its relative path.
// Returns nil if the file type is unrecognized or parsing fails.
func (p *Pipeline) parseFile(relPath string, data []byte) any {
	switch {
	case isDebateMeta(relPath):
		meta, err := parser.ParseDebateMeta(data)
		if err != nil {
			slog.Debug("pipeline: parse debate meta failed", "path", relPath, "error", err)
			return nil
		}
		return meta

	case isScoresFile(relPath):
		entries, skipped := parser.ParseScores(data)
		if skipped > 0 {
			slog.Debug("pipeline: skipped malformed score lines", "path", relPath, "count", skipped)
		}
		return entries

	case isPlanFile(relPath):
		plan, err := parser.ParsePlan(data)
		if err != nil {
			slog.Debug("pipeline: parse plan failed", "path", relPath, "error", err)
			return nil
		}
		return plan

	case isWorkflowFile(relPath):
		wf, err := parser.ParseWorkflow(data)
		if err != nil {
			slog.Debug("pipeline: parse workflow failed", "path", relPath, "error", err)
			return nil
		}
		return wf

	case isProjectFile(relPath):
		proj, err := parser.ParseProject(data)
		if err != nil {
			slog.Debug("pipeline: parse project failed", "path", relPath, "error", err)
			return nil
		}
		return proj
	}

	return nil
}

func isDebateMeta(p string) bool {
	return strings.HasPrefix(p, "debates/") && strings.HasSuffix(p, "/meta.json")
}

func isScoresFile(p string) bool {
	return strings.HasPrefix(p, "scores/")
}

func isPlanFile(p string) bool {
	return p == "plan.yaml" || strings.HasSuffix(p, "/plan.yaml")
}

func isWorkflowFile(p string) bool {
	return p == "workflow.yaml" || strings.HasSuffix(p, "/workflow.yaml")
}

func isProjectFile(p string) bool {
	return p == "project.yaml" || strings.HasSuffix(p, "/project.yaml")
}
