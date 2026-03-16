package pipeline

import (
	"testing"
)

func TestIsDebateMeta(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"debates/foo/meta.json", true},
		{"debates/bar-123/meta.json", true},
		{"debates/meta.json", true}, // edge case: no subdir, but still matches prefix+suffix
		{"other/foo/meta.json", false},
		{"debates/foo/rounds/round-1.md", false},
	}
	for _, tt := range tests {
		if got := isDebateMeta(tt.path); got != tt.want {
			t.Errorf("isDebateMeta(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsPlanFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"plan.yaml", true},
		{"some/nested/plan.yaml", true},
		{"plan.yml", false},
		{"notplan.yaml", false},
	}
	for _, tt := range tests {
		if got := isPlanFile(tt.path); got != tt.want {
			t.Errorf("isPlanFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsWorkflowFile(t *testing.T) {
	if !isWorkflowFile("workflow.yaml") {
		t.Error("expected true for workflow.yaml")
	}
	if isWorkflowFile("plan.yaml") {
		t.Error("expected false for plan.yaml")
	}
}

func TestIsProjectFile(t *testing.T) {
	if !isProjectFile("project.yaml") {
		t.Error("expected true for project.yaml")
	}
	if isProjectFile("workflow.yaml") {
		t.Error("expected false for workflow.yaml")
	}
}

func TestIsScoresFile(t *testing.T) {
	if !isScoresFile("scores/scores.jsonl") {
		t.Error("expected true for scores/scores.jsonl")
	}
	if isScoresFile("debates/foo/meta.json") {
		t.Error("expected false for debates path")
	}
}
