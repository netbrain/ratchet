package pipeline

import (
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/parser"
)

// TestExtractWorkspace verifies workspace name extraction from file paths.
// Only paths starting with "workspaces/" should match; occurrences of
// "workspaces/" deeper in the path must NOT be treated as workspace roots.
func TestExtractWorkspace(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "workspace path with plan.yaml",
			path: "workspaces/monitor/.ratchet/plan.yaml",
			want: "monitor",
		},
		{
			name: "workspace path with debate meta",
			path: "workspaces/api/.ratchet/debates/debate-123/meta.json",
			want: "api",
		},
		{
			name: "workspace path with nested structure",
			path: "workspaces/frontend/.ratchet/guards/build/result.json",
			want: "frontend",
		},
		{
			name: "non-workspace path (root .ratchet)",
			path: ".ratchet/plan.yaml",
			want: "",
		},
		{
			name: "non-workspace path (debates)",
			path: "debates/debate-456/meta.json",
			want: "",
		},
		{
			name: "path without workspaces",
			path: "some/other/path/file.txt",
			want: "",
		},
		{
			name: "workspaces keyword in middle should NOT match",
			path: "data/workspaces/archive/file.txt",
			want: "",
		},
		{
			name: "deeply nested workspaces should NOT match",
			path: "some/deep/workspaces/name/file.txt",
			want: "",
		},
		{
			name: "workspaces with no name after",
			path: "workspaces/",
			want: "",
		},
		{
			name: "minimal valid workspace path",
			path: "workspaces/x",
			want: "x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractWorkspace(tt.path)
			if got != tt.want {
				t.Errorf("extractWorkspace(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestExtractIssue verifies issue reference extraction from debate metadata
// and directory naming conventions.
func TestExtractIssue(t *testing.T) {
	tests := []struct {
		name    string
		relPath string
		meta    *parser.DebateMeta
		want    string
	}{
		{
			name:    "issue from meta.json field",
			relPath: "debates/script-robustness-issue23-20260317T083800/meta.json",
			meta:    &parser.DebateMeta{ID: "script-robustness-issue23-20260317T083800", IssueRef: "#23"},
			want:    "#23",
		},
		{
			name:    "issue from debate ID when meta field empty",
			relPath: "debates/script-robustness-issue23-20260317T083800/meta.json",
			meta:    &parser.DebateMeta{ID: "script-robustness-issue23-20260317T083800"},
			want:    "issue23",
		},
		{
			name:    "issue from debate ID with different pair name",
			relPath: "debates/skill-coherence-issue27-20260317T090000/meta.json",
			meta:    &parser.DebateMeta{ID: "skill-coherence-issue27-20260317T090000"},
			want:    "issue27",
		},
		{
			name:    "no issue in meta or directory name",
			relPath: "debates/script-robustness-20260317T075252/meta.json",
			meta:    &parser.DebateMeta{ID: "script-robustness-20260317T075252"},
			want:    "",
		},
		{
			name:    "nil meta",
			relPath: "debates/foo/meta.json",
			meta:    nil,
			want:    "",
		},
		{
			name:    "issue from debate ID when relPath lacks directory",
			relPath: "debates/meta.json",
			meta:    &parser.DebateMeta{ID: "pair-issue42-20260317T120000"},
			want:    "issue42",
		},
		{
			name:    "multi-digit issue number",
			relPath: "debates/pair-issue123-20260317T120000/meta.json",
			meta:    &parser.DebateMeta{ID: "pair-issue123-20260317T120000"},
			want:    "issue123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIssue(tt.relPath, tt.meta)
			if got != tt.want {
				t.Errorf("extractIssue(%q, meta) = %q, want %q", tt.relPath, got, tt.want)
			}
		})
	}
}
