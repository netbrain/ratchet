package client

import (
	"encoding/json"
	"testing"
)

func TestIssueJSONRoundTrip(t *testing.T) {
	branch := "feat/issue-1-1"
	original := Issue{
		Ref:       "issue-1-1",
		Title:     "Add user authentication",
		Pairs:     []string{"security-review", "api-design"},
		DependsOn: []string{"issue-1-0"},
		PhaseStatus: map[string]string{
			"design": "done",
			"build":  "active",
			"verify": "pending",
		},
		Files:   []string{"auth.go", "auth_test.go"},
		Debates: []string{"debate-001", "debate-002"},
		Branch:  &branch,
		Status:  "active",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal Issue: %v", err)
	}

	var decoded Issue
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal Issue: %v", err)
	}

	if decoded.Ref != original.Ref {
		t.Errorf("Ref: got %q, want %q", decoded.Ref, original.Ref)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title: got %q, want %q", decoded.Title, original.Title)
	}
	if len(decoded.Pairs) != 2 || decoded.Pairs[0] != "security-review" {
		t.Errorf("Pairs: got %v, want %v", decoded.Pairs, original.Pairs)
	}
	if len(decoded.DependsOn) != 1 || decoded.DependsOn[0] != "issue-1-0" {
		t.Errorf("DependsOn: got %v, want %v", decoded.DependsOn, original.DependsOn)
	}
	if decoded.PhaseStatus["build"] != "active" {
		t.Errorf("PhaseStatus[build]: got %q, want %q", decoded.PhaseStatus["build"], "active")
	}
	if len(decoded.Files) != 2 || decoded.Files[0] != "auth.go" {
		t.Errorf("Files: got %v, want %v", decoded.Files, original.Files)
	}
	if len(decoded.Debates) != 2 || decoded.Debates[0] != "debate-001" {
		t.Errorf("Debates: got %v, want %v", decoded.Debates, original.Debates)
	}
	if decoded.Branch == nil || *decoded.Branch != branch {
		t.Errorf("Branch: got %v, want %q", decoded.Branch, branch)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, original.Status)
	}
}

func TestIssueNilBranch(t *testing.T) {
	original := Issue{
		Ref:    "issue-2-1",
		Title:  "No branch yet",
		Status: "pending",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Issue
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Branch != nil {
		t.Errorf("Branch: expected nil, got %v", decoded.Branch)
	}
}

func TestMilestoneWithIssues(t *testing.T) {
	ms := Milestone{
		ID:          1,
		Name:        "Foundation",
		Description: "Set up the base",
		Status:      "active",
		DoneWhen:    "all issues done",
		DependsOn:   []int{},
		Regressions: 0,
		Issues: []Issue{
			{Ref: "issue-1-1", Title: "First issue", Status: "done"},
			{Ref: "issue-1-2", Title: "Second issue", Status: "active"},
		},
	}

	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatalf("Marshal Milestone: %v", err)
	}

	var decoded Milestone
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal Milestone: %v", err)
	}

	if len(decoded.Issues) != 2 {
		t.Fatalf("Issues count: got %d, want 2", len(decoded.Issues))
	}
	if decoded.Issues[0].Ref != "issue-1-1" {
		t.Errorf("Issues[0].Ref: got %q, want %q", decoded.Issues[0].Ref, "issue-1-1")
	}
	if decoded.Issues[1].Status != "active" {
		t.Errorf("Issues[1].Status: got %q, want %q", decoded.Issues[1].Status, "active")
	}
}

func TestMilestoneIssuesJSONKey(t *testing.T) {
	// Verify that the JSON key for Issues is "issues" (matching parser/API).
	ms := Milestone{
		ID: 1,
		Issues: []Issue{
			{Ref: "issue-1-1", Title: "Test", Status: "active"},
		},
	}

	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	if _, ok := raw["issues"]; !ok {
		t.Error("expected JSON key 'issues' in Milestone serialization")
	}
}

func TestStatusInfoIssueRef(t *testing.T) {
	info := StatusInfo{
		MilestoneID:   3,
		MilestoneName: "Polish",
		IssueRef:      "issue-3-2",
		Phase:         "verify",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal StatusInfo: %v", err)
	}

	var decoded StatusInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal StatusInfo: %v", err)
	}

	if decoded.IssueRef != "issue-3-2" {
		t.Errorf("IssueRef: got %q, want %q", decoded.IssueRef, "issue-3-2")
	}
}

func TestStatusInfoIssueRefJSONKey(t *testing.T) {
	info := StatusInfo{
		MilestoneID: 1,
		IssueRef:    "issue-1-1",
		Phase:       "build",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	if _, ok := raw["issue_ref"]; !ok {
		t.Error("expected JSON key 'issue_ref' in StatusInfo serialization")
	}
}

func TestIssueJSONKeysMatchParser(t *testing.T) {
	// Verify all JSON keys match what the parser/API produces.
	branch := "main"
	issue := Issue{
		Ref:         "issue-1-1",
		Title:       "Test",
		Pairs:       []string{"p1"},
		DependsOn:   []string{"issue-1-0"},
		PhaseStatus: map[string]string{"build": "done"},
		Files:       []string{"f.go"},
		Debates:     []string{"d1"},
		Branch:      &branch,
		Status:      "done",
	}

	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	expectedKeys := []string{"ref", "title", "pairs", "depends_on", "phase_status", "files", "debates", "branch", "status"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing JSON key %q in Issue serialization", key)
		}
	}
}
