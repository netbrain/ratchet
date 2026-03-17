package pipeline

import "testing"

// TestExtractWorkspace verifies workspace name extraction from file paths.
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
			name: "workspaces keyword in middle (not actual workspace)",
			path: "data/workspaces/archive/file.txt",
			want: "archive",
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
