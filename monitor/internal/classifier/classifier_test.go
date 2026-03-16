package classifier

import (
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/events"
	"github.com/netbrain/ratchet-monitor/internal/parser"
)

func TestClassify_DebateStarted(t *testing.T) {
	c := New()
	meta := &parser.DebateMeta{Status: "initiated"}
	got := c.Classify("debates/foo/meta.json", meta, events.FileCreated)
	if got != events.DebateStarted {
		t.Errorf("got %q, want %q", got, events.DebateStarted)
	}
}

func TestClassify_DebateResolved(t *testing.T) {
	c := New()
	meta := &parser.DebateMeta{Status: "consensus"}
	got := c.Classify("debates/foo/meta.json", meta, events.FileModified)
	if got != events.DebateResolved {
		t.Errorf("got %q, want %q", got, events.DebateResolved)
	}
}

func TestClassify_DebateUpdated(t *testing.T) {
	c := New()
	meta := &parser.DebateMeta{Status: "in_progress"}
	got := c.Classify("debates/foo/meta.json", meta, events.FileModified)
	if got != events.DebateUpdated {
		t.Errorf("got %q, want %q", got, events.DebateUpdated)
	}
}

func TestClassify_ScoreUpdated(t *testing.T) {
	c := New()
	got := c.Classify("scores/scores.jsonl", nil, events.FileModified)
	if got != events.ScoreUpdated {
		t.Errorf("got %q, want %q", got, events.ScoreUpdated)
	}
}

func TestClassify_PlanUpdated(t *testing.T) {
	c := New()
	got := c.Classify("plan.yaml", nil, events.FileModified)
	if got != events.PlanUpdated {
		t.Errorf("got %q, want %q", got, events.PlanUpdated)
	}
}

func TestClassify_ConfigChanged_WorkflowYaml(t *testing.T) {
	c := New()
	got := c.Classify("workflow.yaml", nil, events.FileModified)
	if got != events.ConfigChanged {
		t.Errorf("got %q, want %q", got, events.ConfigChanged)
	}
}

func TestClassify_ConfigChanged_ProjectYaml(t *testing.T) {
	c := New()
	got := c.Classify("project.yaml", nil, events.FileModified)
	if got != events.ConfigChanged {
		t.Errorf("got %q, want %q", got, events.ConfigChanged)
	}
}

func TestClassify_PairModified(t *testing.T) {
	c := New()
	got := c.Classify("pairs/sse-correctness/generative.md", nil, events.FileModified)
	if got != events.PairModified {
		t.Errorf("got %q, want %q", got, events.PairModified)
	}
}

func TestClassify_UnknownPath_FallsBackToRawType(t *testing.T) {
	c := New()
	got := c.Classify("some/unknown/file.txt", nil, events.FileCreated)
	if got != events.FileCreated {
		t.Errorf("got %q, want fallback %q", got, events.FileCreated)
	}
}

func TestClassify_DebateRoundFile(t *testing.T) {
	c := New()
	got := c.Classify("debates/foo/rounds/round-1-generative.md", nil, events.FileCreated)
	if got != events.DebateUpdated {
		t.Errorf("got %q, want %q", got, events.DebateUpdated)
	}
}

// --- Hardening: edge cases ---

func TestClassify_EmptyPath(t *testing.T) {
	c := New()
	got := c.Classify("", nil, events.FileCreated)
	if got != events.FileCreated {
		t.Errorf("empty path should return fallback, got %q", got)
	}
}

func TestClassify_WindowsBackslashes(t *testing.T) {
	c := New()
	got := c.Classify(`debates\foo\meta.json`, &parser.DebateMeta{Status: "initiated"}, events.FileCreated)
	if got != events.DebateStarted {
		t.Errorf("backslash path should be normalized, got %q", got)
	}
}

func TestClassify_DebateMetaNilContent_FileCreated(t *testing.T) {
	c := New()
	got := c.Classify("debates/foo/meta.json", nil, events.FileCreated)
	if got != events.DebateStarted {
		t.Errorf("nil content + FileCreated fallback should be DebateStarted, got %q", got)
	}
}

func TestClassify_DebateMetaNilContent_FileModified(t *testing.T) {
	c := New()
	got := c.Classify("debates/foo/meta.json", nil, events.FileModified)
	if got != events.DebateUpdated {
		t.Errorf("nil content + FileModified fallback should be DebateUpdated, got %q", got)
	}
}

func TestClassify_DebateMetaWrongContentType(t *testing.T) {
	c := New()
	// Pass a string instead of *DebateMeta — should not panic.
	got := c.Classify("debates/foo/meta.json", "not a DebateMeta", events.FileModified)
	if got != events.DebateUpdated {
		t.Errorf("wrong content type should fall through to non-content branch, got %q", got)
	}
}

func TestClassify_DebateEscalated(t *testing.T) {
	c := New()
	meta := &parser.DebateMeta{Status: "escalated"}
	got := c.Classify("debates/foo/meta.json", meta, events.FileModified)
	if got != events.DebateResolved {
		t.Errorf("escalated should resolve to DebateResolved, got %q", got)
	}
}

func TestClassify_NestedPlanYaml(t *testing.T) {
	c := New()
	got := c.Classify("some/nested/plan.yaml", nil, events.FileModified)
	if got != events.PlanUpdated {
		t.Errorf("nested plan.yaml should be PlanUpdated, got %q", got)
	}
}

func TestClassify_TableDriven(t *testing.T) {
	c := New()

	tests := []struct {
		name     string
		path     string
		content  any
		fallback events.EventType
		want     events.EventType
	}{
		{
			name:     "debate meta initiated",
			path:     "debates/test/meta.json",
			content:  &parser.DebateMeta{Status: "initiated"},
			fallback: events.FileCreated,
			want:     events.DebateStarted,
		},
		{
			name:     "debate meta consensus",
			path:     "debates/test/meta.json",
			content:  &parser.DebateMeta{Status: "consensus"},
			fallback: events.FileModified,
			want:     events.DebateResolved,
		},
		{
			name:     "scores file",
			path:     "scores/scores.jsonl",
			content:  nil,
			fallback: events.FileModified,
			want:     events.ScoreUpdated,
		},
		{
			name:     "plan file",
			path:     "plan.yaml",
			content:  nil,
			fallback: events.FileModified,
			want:     events.PlanUpdated,
		},
		{
			name:     "workflow config",
			path:     "workflow.yaml",
			content:  nil,
			fallback: events.FileModified,
			want:     events.ConfigChanged,
		},
		{
			name:     "project config",
			path:     "project.yaml",
			content:  nil,
			fallback: events.FileModified,
			want:     events.ConfigChanged,
		},
		{
			name:     "pair md file",
			path:     "pairs/go-idioms/adversarial.md",
			content:  nil,
			fallback: events.FileModified,
			want:     events.PairModified,
		},
		{
			name:     "unknown file",
			path:     "random/stuff.txt",
			content:  nil,
			fallback: events.FileDeleted,
			want:     events.FileDeleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Classify(tt.path, tt.content, tt.fallback)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
