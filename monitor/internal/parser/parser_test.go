package parser

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(testdataPath(name))
	if err != nil {
		t.Fatalf("failed to read testdata/%s: %v", name, err)
	}
	return data
}

// --- WorkflowConfig tests ---

func TestParseWorkflow_Valid(t *testing.T) {
	data := readTestdata(t, "workflow.yaml")
	wf, err := ParseWorkflow(data)
	if err != nil {
		t.Fatalf("ParseWorkflow returned error: %v", err)
	}

	if wf.Version != 2 {
		t.Errorf("Version: got %d, want 2", wf.Version)
	}
	if wf.MaxRounds != 3 {
		t.Errorf("MaxRounds: got %d, want 3", wf.MaxRounds)
	}
	if wf.Escalation != "human" {
		t.Errorf("Escalation: got %q, want %q", wf.Escalation, "human")
	}
	if wf.Progress.Adapter != "none" {
		t.Errorf("Progress.Adapter: got %q, want %q", wf.Progress.Adapter, "none")
	}
	if len(wf.Components) != 2 {
		t.Fatalf("Components: got %d, want 2", len(wf.Components))
	}
	if wf.Components[0].Name != "backend" {
		t.Errorf("Components[0].Name: got %q, want %q", wf.Components[0].Name, "backend")
	}
	if wf.Components[0].Workflow != "tdd" {
		t.Errorf("Components[0].Workflow: got %q, want %q", wf.Components[0].Workflow, "tdd")
	}
	if len(wf.Pairs) != 2 {
		t.Fatalf("Pairs: got %d, want 2", len(wf.Pairs))
	}
	if wf.Pairs[0].Name != "api-design" {
		t.Errorf("Pairs[0].Name: got %q, want %q", wf.Pairs[0].Name, "api-design")
	}
	if !wf.Pairs[0].Enabled {
		t.Error("Pairs[0].Enabled: got false, want true")
	}
	if len(wf.Guards) != 2 {
		t.Fatalf("Guards: got %d, want 2", len(wf.Guards))
	}
	if wf.Guards[0].Name != "format" {
		t.Errorf("Guards[0].Name: got %q, want %q", wf.Guards[0].Name, "format")
	}
	if wf.Guards[0].Expect != "empty" {
		t.Errorf("Guards[0].Expect: got %q, want %q", wf.Guards[0].Expect, "empty")
	}
}

func TestParseWorkflow_Malformed(t *testing.T) {
	data := readTestdata(t, "malformed.yaml")
	_, err := ParseWorkflow(data)
	if err == nil {
		t.Error("ParseWorkflow should return error for malformed YAML")
	}
}

func TestParseWorkflow_Empty(t *testing.T) {
	_, err := ParseWorkflow([]byte{})
	if err != nil {
		t.Errorf("ParseWorkflow with empty input should not error, got: %v", err)
	}
}

// --- Plan tests ---

func TestParsePlan_Valid(t *testing.T) {
	data := readTestdata(t, "plan.yaml")
	plan, err := ParsePlan(data)
	if err != nil {
		t.Fatalf("ParsePlan returned error: %v", err)
	}

	if plan.Epic.Name != "ratchet-monitor" {
		t.Errorf("Epic.Name: got %q, want %q", plan.Epic.Name, "ratchet-monitor")
	}
	if len(plan.Epic.Milestones) != 2 {
		t.Fatalf("Milestones: got %d, want 2", len(plan.Epic.Milestones))
	}

	m1 := plan.Epic.Milestones[0]
	if m1.ID != 1 {
		t.Errorf("Milestone[0].ID: got %d, want 1", m1.ID)
	}
	if m1.Status != "done" {
		t.Errorf("Milestone[0].Status: got %q, want %q", m1.Status, "done")
	}
	if m1.PhaseStatus["plan"] != "done" {
		t.Errorf("Milestone[0].PhaseStatus[plan]: got %q, want %q", m1.PhaseStatus["plan"], "done")
	}
	if len(m1.Pairs) != 3 {
		t.Errorf("Milestone[0].Pairs: got %d, want 3", len(m1.Pairs))
	}

	m2 := plan.Epic.Milestones[1]
	if m2.Status != "in_progress" {
		t.Errorf("Milestone[1].Status: got %q, want %q", m2.Status, "in_progress")
	}
	if m2.PhaseStatus["test"] != "in_progress" {
		t.Errorf("Milestone[1].PhaseStatus[test]: got %q, want %q", m2.PhaseStatus["test"], "in_progress")
	}

	if plan.Epic.CurrentFocus == nil {
		t.Fatal("CurrentFocus is nil")
	}
	if plan.Epic.CurrentFocus.MilestoneID != 2 {
		t.Errorf("CurrentFocus.MilestoneID: got %d, want 2", plan.Epic.CurrentFocus.MilestoneID)
	}
	if plan.Epic.CurrentFocus.Phase != "test" {
		t.Errorf("CurrentFocus.Phase: got %q, want %q", plan.Epic.CurrentFocus.Phase, "test")
	}
}

func TestParsePlan_Malformed(t *testing.T) {
	data := readTestdata(t, "malformed.yaml")
	_, err := ParsePlan(data)
	if err == nil {
		t.Error("ParsePlan should return error for malformed YAML")
	}
}

func TestParsePlan_Empty(t *testing.T) {
	_, err := ParsePlan([]byte{})
	if err != nil {
		t.Errorf("ParsePlan with empty input should not error, got: %v", err)
	}
}

// --- ProjectConfig tests ---

func TestParseProject_Valid(t *testing.T) {
	data := readTestdata(t, "project.yaml")
	proj, err := ParseProject(data)
	if err != nil {
		t.Fatalf("ParseProject returned error: %v", err)
	}

	if proj.Name != "ratchet-monitor" {
		t.Errorf("Name: got %q, want %q", proj.Name, "ratchet-monitor")
	}
	if proj.Stack.Language != "go" {
		t.Errorf("Stack.Language: got %q, want %q", proj.Stack.Language, "go")
	}
	if proj.Stack.Frontend != "alpine.js" {
		t.Errorf("Stack.Frontend: got %q, want %q", proj.Stack.Frontend, "alpine.js")
	}
	if proj.Stack.Transport != "sse" {
		t.Errorf("Stack.Transport: got %q, want %q", proj.Stack.Transport, "sse")
	}
	if proj.Architecture.Pattern != "single-binary-web-server" {
		t.Errorf("Architecture.Pattern: got %q, want %q", proj.Architecture.Pattern, "single-binary-web-server")
	}
	if len(proj.Architecture.Components) != 2 {
		t.Fatalf("Architecture.Components: got %d, want 2", len(proj.Architecture.Components))
	}
	if proj.Testing.Framework != "go-test" {
		t.Errorf("Testing.Framework: got %q, want %q", proj.Testing.Framework, "go-test")
	}
	if proj.Testing.Commands["unit"] != "go test -race ./..." {
		t.Errorf("Testing.Commands[unit]: got %q", proj.Testing.Commands["unit"])
	}
}

func TestParseProject_Malformed(t *testing.T) {
	data := readTestdata(t, "malformed.yaml")
	_, err := ParseProject(data)
	if err == nil {
		t.Error("ParseProject should return error for malformed YAML")
	}
}

func TestParseProject_Empty(t *testing.T) {
	_, err := ParseProject([]byte{})
	if err != nil {
		t.Errorf("ParseProject with empty input should not error, got: %v", err)
	}
}

// --- DebateMeta tests ---

func TestParseDebateMeta_Valid(t *testing.T) {
	data := readTestdata(t, "meta.json")
	meta, err := ParseDebateMeta(data)
	if err != nil {
		t.Fatalf("ParseDebateMeta returned error: %v", err)
	}

	if meta.ID != "api-design-20260313T164500" {
		t.Errorf("ID: got %q, want %q", meta.ID, "api-design-20260313T164500")
	}
	if meta.Pair != "api-design" {
		t.Errorf("Pair: got %q, want %q", meta.Pair, "api-design")
	}
	if meta.Phase != "review" {
		t.Errorf("Phase: got %q, want %q", meta.Phase, "review")
	}
	if meta.Milestone != 1 {
		t.Errorf("Milestone: got %d, want 1", meta.Milestone)
	}
	if meta.Status != "consensus" {
		t.Errorf("Status: got %q, want %q", meta.Status, "consensus")
	}
	if meta.RoundCount != 1 {
		t.Errorf("RoundCount: got %d, want 1", meta.RoundCount)
	}
	if meta.MaxRounds != 3 {
		t.Errorf("MaxRounds: got %d, want 3", meta.MaxRounds)
	}
	if meta.Verdict == nil || *meta.Verdict != "ACCEPT" {
		t.Errorf("Verdict: got %v, want ACCEPT", meta.Verdict)
	}
	if meta.Resolved == nil {
		t.Error("Resolved should not be nil for consensus status")
	}
}

func TestParseDebateMeta_Initiated(t *testing.T) {
	data := readTestdata(t, "meta_initiated.json")
	meta, err := ParseDebateMeta(data)
	if err != nil {
		t.Fatalf("ParseDebateMeta returned error: %v", err)
	}

	if meta.Status != "initiated" {
		t.Errorf("Status: got %q, want %q", meta.Status, "initiated")
	}
	if meta.RoundCount != 0 {
		t.Errorf("RoundCount: got %d, want 0", meta.RoundCount)
	}
	if meta.Resolved != nil {
		t.Errorf("Resolved should be nil for initiated status, got %v", meta.Resolved)
	}
	if meta.Verdict != nil {
		t.Errorf("Verdict should be nil for initiated status, got %v", meta.Verdict)
	}
	if len(meta.Files) != 1 {
		t.Errorf("Files: got %d, want 1", len(meta.Files))
	}
}

func TestParseDebateMeta_Malformed(t *testing.T) {
	data := readTestdata(t, "malformed.json")
	_, err := ParseDebateMeta(data)
	if err == nil {
		t.Error("ParseDebateMeta should return error for malformed JSON")
	}
}

func TestParseDebateMeta_Empty(t *testing.T) {
	_, err := ParseDebateMeta([]byte{})
	if err == nil {
		t.Error("ParseDebateMeta should return error for empty input")
	}
}

// --- ScoreEntry tests ---

func TestParseScoreEntry_Valid(t *testing.T) {
	line := `{"timestamp":"2026-03-12T18:30:00Z","debate_id":"prompt-coherence-20260312T180000","pair":"prompt-coherence","milestone":2,"rounds_to_consensus":2,"escalated":false,"issues_found":9,"issues_resolved":9}`
	entry, err := ParseScoreEntry([]byte(line))
	if err != nil {
		t.Fatalf("ParseScoreEntry returned error: %v", err)
	}

	if entry.DebateID != "prompt-coherence-20260312T180000" {
		t.Errorf("DebateID: got %q", entry.DebateID)
	}
	if entry.Pair != "prompt-coherence" {
		t.Errorf("Pair: got %q", entry.Pair)
	}
	if entry.Milestone != 2 {
		t.Errorf("Milestone: got %d, want 2", entry.Milestone)
	}
	if entry.RoundsToConsensus != 2 {
		t.Errorf("RoundsToConsensus: got %d, want 2", entry.RoundsToConsensus)
	}
	if entry.Escalated {
		t.Error("Escalated: got true, want false")
	}
	if entry.IssuesFound != 9 {
		t.Errorf("IssuesFound: got %d, want 9", entry.IssuesFound)
	}
	if entry.IssuesResolved != 9 {
		t.Errorf("IssuesResolved: got %d, want 9", entry.IssuesResolved)
	}
}

func TestParseScoreEntry_Malformed(t *testing.T) {
	_, err := ParseScoreEntry([]byte("{bad json"))
	if err == nil {
		t.Error("ParseScoreEntry should return error for malformed JSON")
	}
}

func TestParseScoreEntry_Empty(t *testing.T) {
	_, err := ParseScoreEntry([]byte{})
	if err == nil {
		t.Error("ParseScoreEntry should return error for empty input")
	}
}

func TestParseScores_Valid(t *testing.T) {
	data := readTestdata(t, "scores.jsonl")
	entries, skipped := ParseScores(data)
	if skipped != 0 {
		t.Fatalf("ParseScores skipped %d lines, want 0", skipped)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Pair != "prompt-coherence" {
		t.Errorf("entries[0].Pair: got %q", entries[0].Pair)
	}
	if entries[1].Pair != "script-integrity" {
		t.Errorf("entries[1].Pair: got %q", entries[1].Pair)
	}
}

func TestParseScores_Empty(t *testing.T) {
	entries, skipped := ParseScores([]byte{})
	if skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", skipped)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// --- PairDefinition tests ---

func TestParsePairDefinition_Valid(t *testing.T) {
	data := readTestdata(t, "pair_generative.md")
	pd, err := ParsePairDefinition(data)
	if err != nil {
		t.Fatalf("ParsePairDefinition returned error: %v", err)
	}

	if pd.Name != "SSE Correctness — Generative Agent" {
		t.Errorf("Name: got %q, want %q", pd.Name, "SSE Correctness — Generative Agent")
	}
	if pd.Content == "" {
		t.Error("Content should not be empty")
	}
	if len(pd.Content) < 50 {
		t.Errorf("Content too short: %d bytes", len(pd.Content))
	}
}

func TestParsePairDefinition_NoHeading(t *testing.T) {
	data := []byte("No heading here, just some text.\nMore text.")
	pd, err := ParsePairDefinition(data)
	if err != nil {
		t.Fatalf("ParsePairDefinition returned error: %v", err)
	}

	if pd.Name != "" {
		t.Errorf("Name should be empty when no H1 heading, got %q", pd.Name)
	}
	if pd.Content == "" {
		t.Error("Content should not be empty even without heading")
	}
}

func TestParsePairDefinition_Empty(t *testing.T) {
	pd, err := ParsePairDefinition([]byte{})
	if err != nil {
		t.Errorf("ParsePairDefinition with empty input should not error, got: %v", err)
	}
	if pd.Name != "" {
		t.Errorf("Name should be empty for empty input, got %q", pd.Name)
	}
	if pd.Content != "" {
		t.Errorf("Content should be empty for empty input, got %q", pd.Content)
	}
}

// --- ParseRoundFilename tests ---

func TestParseRoundFilename_Valid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNum  int
		wantRole string
	}{
		{"generative round 1", "round-1-generative.md", 1, "generative"},
		{"adversarial round 2", "round-2-adversarial.md", 2, "adversarial"},
		{"round 0", "round-0-generative.md", 0, "generative"},
		{"large number", "round-42-reviewer.md", 42, "reviewer"},
		{"hyphenated role", "round-3-co-author.md", 3, "co-author"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, role, err := ParseRoundFilename(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if num != tt.wantNum {
				t.Errorf("number: got %d, want %d", num, tt.wantNum)
			}
			if role != tt.wantRole {
				t.Errorf("role: got %q, want %q", role, tt.wantRole)
			}
		})
	}
}

func TestParseRoundFilename_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no .md extension", "round-1-generative.txt"},
		{"no round prefix", "file-1-generative.md"},
		{"missing role", "round-1.md"},
		{"not a number", "round-abc-generative.md"},
		{"empty role", "round-1-.md"},
		{"negative number", "round--1-generative.md"},
		{"empty filename", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseRoundFilename(tt.input)
			if err == nil {
				t.Error("expected error for invalid input")
			}
		})
	}
}

// --- Hardening: sentinel error tests ---

func TestParseDebateMeta_Empty_WrapsErrEmptyInput(t *testing.T) {
	_, err := ParseDebateMeta([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !errors.Is(err, ErrEmptyInput) {
		t.Errorf("error should wrap ErrEmptyInput, got: %v", err)
	}
}

func TestParseScoreEntry_Empty_WrapsErrEmptyInput(t *testing.T) {
	_, err := ParseScoreEntry([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !errors.Is(err, ErrEmptyInput) {
		t.Errorf("error should wrap ErrEmptyInput, got: %v", err)
	}
}

// --- Hardening: ParseScores skip malformed lines ---

func TestParseScores_MalformedLine_SkipsAndCounts(t *testing.T) {
	data := []byte(`{"timestamp":"2026-03-12T18:30:00Z","debate_id":"ok","pair":"ok","milestone":1,"rounds_to_consensus":1,"escalated":false,"issues_found":1,"issues_resolved":1}
{bad json}
`)
	entries, skipped := ParseScores(data)
	if skipped != 1 {
		t.Errorf("expected 1 skipped line, got %d", skipped)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 valid entry, got %d", len(entries))
	}
	if len(entries) > 0 && entries[0].DebateID != "ok" {
		t.Errorf("expected debate_id 'ok', got %q", entries[0].DebateID)
	}
}

func TestParseScores_BlankLines(t *testing.T) {
	data := []byte(`
{"timestamp":"2026-03-12T18:30:00Z","debate_id":"d1","pair":"p1","milestone":1,"rounds_to_consensus":1,"escalated":false,"issues_found":1,"issues_resolved":1}

{"timestamp":"2026-03-12T18:31:00Z","debate_id":"d2","pair":"p2","milestone":2,"rounds_to_consensus":2,"escalated":true,"issues_found":3,"issues_resolved":2}

`)
	entries, skipped := ParseScores(data)
	if skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", skipped)
	}
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
}

func TestParseScores_AllMalformed(t *testing.T) {
	data := []byte("{bad}\n{also bad}\n")
	entries, skipped := ParseScores(data)
	if skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", skipped)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseScores_MixedValidAndMalformed(t *testing.T) {
	data := []byte(`{"timestamp":"2026-03-12T18:30:00Z","debate_id":"d1","pair":"p1","milestone":1,"rounds_to_consensus":1,"escalated":false,"issues_found":1,"issues_resolved":1}
{truncated
{"timestamp":"2026-03-12T18:31:00Z","debate_id":"d2","pair":"p2","milestone":2,"rounds_to_consensus":2,"escalated":true,"issues_found":3,"issues_resolved":2}
not json at all
`)
	entries, skipped := ParseScores(data)
	if skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", skipped)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 valid entries, got %d", len(entries))
	}
}

// --- Hardening: parser robustness with malformed/partial input ---

func TestParseDebateMeta_PartialJSON(t *testing.T) {
	_, err := ParseDebateMeta([]byte(`{"id": "test"`))
	if err == nil {
		t.Error("expected error for truncated JSON")
	}
}

func TestParseDebateMeta_NullJSON(t *testing.T) {
	_, err := ParseDebateMeta([]byte("null"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseWorkflow_OnlyWhitespace(t *testing.T) {
	wf, err := ParseWorkflow([]byte("   \n\t\n  "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// YAML treats whitespace-only as empty document -> zero-value struct.
	if wf.Version != 0 {
		t.Errorf("expected zero version, got %d", wf.Version)
	}
}

func TestParsePlan_OnlyWhitespace(t *testing.T) {
	p, err := ParsePlan([]byte("  \n  "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Epic.Name != "" {
		t.Errorf("expected empty epic name, got %q", p.Epic.Name)
	}
}

func TestParsePairDefinition_MultipleH1Headings(t *testing.T) {
	data := []byte("# First\n\nSome content.\n\n# Second\n\nMore content.\n")
	pd, err := ParsePairDefinition(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pd.Name != "First" {
		t.Errorf("should use first H1 heading, got %q", pd.Name)
	}
}

func TestParsePairDefinition_H2BeforeH1(t *testing.T) {
	data := []byte("## Subheading\n\n# Main Title\n\nContent.\n")
	pd, err := ParsePairDefinition(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pd.Name != "Main Title" {
		t.Errorf("should find first H1, got %q", pd.Name)
	}
}

func TestParseScoreEntry_ExtraFields(t *testing.T) {
	line := `{"timestamp":"2026-03-12T18:30:00Z","debate_id":"d1","pair":"p1","milestone":1,"rounds_to_consensus":1,"escalated":false,"issues_found":1,"issues_resolved":1,"extra":"ignored"}`
	entry, err := ParseScoreEntry([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Pair != "p1" {
		t.Errorf("Pair: got %q, want %q", entry.Pair, "p1")
	}
}
