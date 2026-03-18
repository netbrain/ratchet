package datasource

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/handler"
	"github.com/netbrain/ratchet-monitor/internal/parser"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// workflow.yaml
	workflow := `version: 2
max_rounds: 3
escalation: human
progress:
  adapter: none
components:
  - name: backend
    scope: internal/
    workflow: tdd
pairs:
  - name: api-design
    component: backend
    phase: review
    scope: internal/handler
    enabled: true
  - name: sse-correctness
    component: backend
    phase: test
    scope: internal/sse
    enabled: true
guards: []
`
	os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(workflow), 0o644)

	// plan.yaml
	plan := `epic:
  name: ratchet-monitor
  description: Real-time observability dashboard
  milestones:
    - id: 1
      name: "Spike"
      description: "Initial spike"
      pairs: ["api-design"]
      status: done
      phase_status:
        plan: done
        build: done
      done_when: "all green"
    - id: 2
      name: "Solid Backend"
      description: "Production backend"
      pairs: ["api-design", "sse-correctness"]
      status: in_progress
      phase_status:
        plan: done
        build: in_progress
      done_when: "tests pass"
  current_focus:
    milestone_id: 2
    phase: build
    started: "2026-03-14"
`
	os.WriteFile(filepath.Join(dir, "plan.yaml"), []byte(plan), 0o644)

	// debates directory with meta.json files
	debate1Dir := filepath.Join(dir, "debates", "api-design-1")
	os.MkdirAll(debate1Dir, 0o755)

	started := time.Date(2026, 3, 13, 16, 45, 0, 0, time.UTC)
	meta1 := map[string]any{
		"id":          "api-design-1",
		"pair":        "api-design",
		"phase":       "review",
		"milestone":   1,
		"files":       []string{"handler.go"},
		"status":      "consensus",
		"round_count": 1,
		"max_rounds":  3,
		"started":     started.Format(time.RFC3339),
		"resolved":    started.Add(time.Hour).Format(time.RFC3339),
		"verdict":     "ACCEPT",
	}
	data1, _ := json.Marshal(meta1)
	os.WriteFile(filepath.Join(debate1Dir, "meta.json"), data1, 0o644)

	debate2Dir := filepath.Join(dir, "debates", "sse-test-1")
	os.MkdirAll(filepath.Join(debate2Dir, "rounds"), 0o755)

	meta2 := map[string]any{
		"id":          "sse-test-1",
		"pair":        "sse-correctness",
		"phase":       "test",
		"milestone":   2,
		"files":       []string{"broker.go"},
		"status":      "in_progress",
		"round_count": 2,
		"max_rounds":  3,
		"started":     started.Add(2 * time.Hour).Format(time.RFC3339),
	}
	data2, _ := json.Marshal(meta2)
	os.WriteFile(filepath.Join(debate2Dir, "meta.json"), data2, 0o644)

	// Round files.
	os.WriteFile(filepath.Join(debate2Dir, "rounds", "round-1-generative.md"), []byte("# Round 1 generative\nContent here."), 0o644)
	os.WriteFile(filepath.Join(debate2Dir, "rounds", "round-1-adversarial.md"), []byte("# Round 1 adversarial\nReview here."), 0o644)
	os.WriteFile(filepath.Join(debate2Dir, "rounds", "round-2-generative.md"), []byte("# Round 2 generative\nMore content."), 0o644)

	return dir
}

func TestFileDataSource_Pairs(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	result, err := ds.Pairs()
	if err != nil {
		t.Fatalf("Pairs() error: %v", err)
	}

	pairs, ok := result.([]PairStatus)
	if !ok {
		t.Fatalf("expected []PairStatus, got %T", result)
	}

	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}

	// api-design should not be active (consensus status)
	for _, p := range pairs {
		if p.Name == "api-design" && p.Active {
			t.Error("api-design should not be active (consensus)")
		}
		if p.Name == "sse-correctness" && !p.Active {
			t.Error("sse-correctness should be active (in_progress)")
		}
	}
}

func TestFileDataSource_Debates(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	result, err := ds.Debates()
	if err != nil {
		t.Fatalf("Debates() error: %v", err)
	}

	debates, ok := result.([]parser.DebateMeta)
	if !ok {
		t.Fatalf("expected []parser.DebateMeta, got %T", result)
	}

	if len(debates) != 2 {
		t.Fatalf("expected 2 debates, got %d", len(debates))
	}

	// Should be sorted by started time descending.
	if debates[0].Started.Before(debates[1].Started) {
		t.Error("debates should be sorted by started time descending")
	}
}

func TestFileDataSource_Debate(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	result, err := ds.Debate("sse-test-1")
	if err != nil {
		t.Fatalf("Debate() error: %v", err)
	}

	dwr, ok := result.(*parser.DebateWithRounds)
	if !ok {
		t.Fatalf("expected *parser.DebateWithRounds, got %T", result)
	}

	if dwr.ID != "sse-test-1" {
		t.Errorf("ID: got %q, want %q", dwr.ID, "sse-test-1")
	}

	if len(dwr.Rounds) != 3 {
		t.Fatalf("expected 3 rounds, got %d", len(dwr.Rounds))
	}

	// Check sort order: generative before adversarial within each round number.
	if dwr.Rounds[0].Number != 1 || dwr.Rounds[0].Role != "generative" {
		t.Errorf("first round: got %d/%s, want 1/generative", dwr.Rounds[0].Number, dwr.Rounds[0].Role)
	}
	if dwr.Rounds[1].Number != 1 || dwr.Rounds[1].Role != "adversarial" {
		t.Errorf("second round: got %d/%s, want 1/adversarial", dwr.Rounds[1].Number, dwr.Rounds[1].Role)
	}
	if dwr.Rounds[2].Number != 2 || dwr.Rounds[2].Role != "generative" {
		t.Errorf("third round: got %d/%s, want 2/generative", dwr.Rounds[2].Number, dwr.Rounds[2].Role)
	}
}

func TestFileDataSource_Debate_NotFound(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	_, err := ds.Debate("nonexistent")
	if err == nil {
		t.Error("Debate() should return error for nonexistent debate")
	}
}

// TestFileDataSource_Debate_NotFound_ReturnsNotFoundError verifies that
// Debate() returns a handler.NotFoundError (not a generic error) for missing
// debates, so the handler can distinguish 404 from 500.
func TestFileDataSource_Debate_NotFound_ReturnsNotFoundError(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	_, err := ds.Debate("nonexistent")
	if err == nil {
		t.Fatal("Debate() should return error for nonexistent debate")
	}

	var nfe *handler.NotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("expected handler.NotFoundError, got %T: %v", err, err)
	}
	if nfe.Resource != "debate" {
		t.Errorf("NotFoundError.Resource: got %q, want %q", nfe.Resource, "debate")
	}
	if nfe.ID != "nonexistent" {
		t.Errorf("NotFoundError.ID: got %q, want %q", nfe.ID, "nonexistent")
	}
}

func TestFileDataSource_Plan(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	result, err := ds.Plan()
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	resp, ok := result.(*PlanResponse)
	if !ok {
		t.Fatalf("expected *PlanResponse, got %T", result)
	}

	if resp.Epic.Name != "ratchet-monitor" {
		t.Errorf("Epic.Name: got %q, want %q", resp.Epic.Name, "ratchet-monitor")
	}
	// workflow.yaml in setupTestDir has no max_regressions → defaults to 2
	if resp.MaxRegressions != 2 {
		t.Errorf("MaxRegressions: got %d, want 2", resp.MaxRegressions)
	}
}

func TestFileDataSource_Status(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	result, err := ds.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	info, ok := result.(*StatusInfo)
	if !ok {
		t.Fatalf("expected *StatusInfo, got %T", result)
	}

	if info.MilestoneID != 2 {
		t.Errorf("MilestoneID: got %d, want 2", info.MilestoneID)
	}
	if info.MilestoneName != "Solid Backend" {
		t.Errorf("MilestoneName: got %q, want %q", info.MilestoneName, "Solid Backend")
	}
	if info.Phase != "build" {
		t.Errorf("Phase: got %q, want %q", info.Phase, "build")
	}
}

func TestFileDataSource_Scores(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	// Create scores directory and file.
	scoresDir := filepath.Join(dir, "scores")
	os.MkdirAll(scoresDir, 0o755)

	lines := `{"timestamp":"2026-03-14T10:00:00Z","debate_id":"d1","pair":"api-design","milestone":1,"rounds_to_consensus":2,"escalated":false,"issues_found":3,"issues_resolved":3}
{"timestamp":"2026-03-15T12:00:00Z","debate_id":"d2","pair":"sse-correctness","milestone":2,"rounds_to_consensus":1,"escalated":false,"issues_found":1,"issues_resolved":1}
{"timestamp":"2026-03-14T14:00:00Z","debate_id":"d3","pair":"api-design","milestone":2,"rounds_to_consensus":3,"escalated":true,"issues_found":5,"issues_resolved":2}
`
	os.WriteFile(filepath.Join(scoresDir, "scores.jsonl"), []byte(lines), 0o644)

	// All scores, sorted by timestamp descending.
	result, err := ds.Scores("")
	if err != nil {
		t.Fatalf("Scores() error: %v", err)
	}
	entries, ok := result.([]parser.ScoreEntry)
	if !ok {
		t.Fatalf("expected []parser.ScoreEntry, got %T", result)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Most recent first.
	if entries[0].DebateID != "d2" {
		t.Errorf("first entry should be d2 (most recent), got %s", entries[0].DebateID)
	}
	if entries[2].DebateID != "d1" {
		t.Errorf("last entry should be d1 (oldest), got %s", entries[2].DebateID)
	}
}

func TestFileDataSource_Scores_FilterByPair(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	scoresDir := filepath.Join(dir, "scores")
	os.MkdirAll(scoresDir, 0o755)

	lines := `{"timestamp":"2026-03-14T10:00:00Z","debate_id":"d1","pair":"api-design","milestone":1,"rounds_to_consensus":2,"escalated":false,"issues_found":3,"issues_resolved":3}
{"timestamp":"2026-03-15T12:00:00Z","debate_id":"d2","pair":"sse-correctness","milestone":2,"rounds_to_consensus":1,"escalated":false,"issues_found":1,"issues_resolved":1}
`
	os.WriteFile(filepath.Join(scoresDir, "scores.jsonl"), []byte(lines), 0o644)

	result, err := ds.Scores("api-design")
	if err != nil {
		t.Fatalf("Scores(api-design) error: %v", err)
	}
	entries, ok := result.([]parser.ScoreEntry)
	if !ok {
		t.Fatalf("expected []parser.ScoreEntry, got %T", result)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for api-design, got %d", len(entries))
	}
	if entries[0].Pair != "api-design" {
		t.Errorf("expected pair api-design, got %s", entries[0].Pair)
	}
}

func TestFileDataSource_Scores_NoFile(t *testing.T) {
	dir := t.TempDir()
	ds := NewFileDataSource(dir)

	result, err := ds.Scores("")
	if err != nil {
		t.Fatalf("Scores() should not error when file is missing: %v", err)
	}
	entries, ok := result.([]parser.ScoreEntry)
	if !ok {
		t.Fatalf("expected []parser.ScoreEntry, got %T", result)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}
}

func TestFileDataSource_Scores_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	scoresDir := filepath.Join(dir, "scores")
	os.MkdirAll(scoresDir, 0o755)
	os.WriteFile(filepath.Join(scoresDir, "scores.jsonl"), []byte(""), 0o644)

	ds := NewFileDataSource(dir)
	result, err := ds.Scores("")
	if err != nil {
		t.Fatalf("Scores() should not error on empty file: %v", err)
	}
	entries, ok := result.([]parser.ScoreEntry)
	if !ok {
		t.Fatalf("expected []parser.ScoreEntry, got %T", result)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}
}

func TestFileDataSource_Scores_FileTooLarge(t *testing.T) {
	dir := t.TempDir()
	scoresDir := filepath.Join(dir, "scores")
	os.MkdirAll(scoresDir, 0o755)

	// Create a file that exceeds maxScoresFileSize (10 MiB).
	f, err := os.Create(filepath.Join(scoresDir, "scores.jsonl"))
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	// Write just over the limit. We don't need valid JSONL — the size check
	// happens before parsing.
	if err := f.Truncate(maxScoresFileSize + 1); err != nil {
		f.Close()
		t.Fatalf("truncate: %v", err)
	}
	f.Close()

	ds := NewFileDataSource(dir)
	_, err = ds.Scores("")
	if err == nil {
		t.Fatal("Scores() should return error for oversized file")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error should mention 'too large', got: %v", err)
	}
}

func TestFileDataSource_Scores_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	scoresDir := filepath.Join(dir, "scores")
	os.MkdirAll(scoresDir, 0o755)

	// Mix of valid and malformed lines.
	lines := `{"timestamp":"2026-03-14T10:00:00Z","debate_id":"d1","pair":"api-design","milestone":1,"rounds_to_consensus":2,"escalated":false,"issues_found":3,"issues_resolved":3}
{bad json here}
{"timestamp":"2026-03-15T12:00:00Z","debate_id":"d2","pair":"sse-correctness","milestone":2,"rounds_to_consensus":1,"escalated":false,"issues_found":1,"issues_resolved":1}
also not json
`
	os.WriteFile(filepath.Join(scoresDir, "scores.jsonl"), []byte(lines), 0o644)

	ds := NewFileDataSource(dir)
	result, err := ds.Scores("")
	if err != nil {
		t.Fatalf("Scores() should not error with malformed lines: %v", err)
	}
	entries, ok := result.([]parser.ScoreEntry)
	if !ok {
		t.Fatalf("expected []parser.ScoreEntry, got %T", result)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 valid entries (skipping 2 malformed), got %d", len(entries))
	}
}

func TestFileDataSource_Scores_FilterByPairWithMalformedLines(t *testing.T) {
	dir := t.TempDir()
	scoresDir := filepath.Join(dir, "scores")
	os.MkdirAll(scoresDir, 0o755)

	lines := `{"timestamp":"2026-03-14T10:00:00Z","debate_id":"d1","pair":"api-design","milestone":1,"rounds_to_consensus":2,"escalated":false,"issues_found":3,"issues_resolved":3}
{malformed}
{"timestamp":"2026-03-15T12:00:00Z","debate_id":"d2","pair":"sse-correctness","milestone":2,"rounds_to_consensus":1,"escalated":false,"issues_found":1,"issues_resolved":1}
`
	os.WriteFile(filepath.Join(scoresDir, "scores.jsonl"), []byte(lines), 0o644)

	ds := NewFileDataSource(dir)
	result, err := ds.Scores("api-design")
	if err != nil {
		t.Fatalf("Scores(api-design) error: %v", err)
	}
	entries, ok := result.([]parser.ScoreEntry)
	if !ok {
		t.Fatalf("expected []parser.ScoreEntry, got %T", result)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry for api-design, got %d", len(entries))
	}
}

func TestFileDataSource_Scores_NonexistentPairReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	scoresDir := filepath.Join(dir, "scores")
	os.MkdirAll(scoresDir, 0o755)

	lines := `{"timestamp":"2026-03-14T10:00:00Z","debate_id":"d1","pair":"api-design","milestone":1,"rounds_to_consensus":2,"escalated":false,"issues_found":3,"issues_resolved":3}
`
	os.WriteFile(filepath.Join(scoresDir, "scores.jsonl"), []byte(lines), 0o644)

	ds := NewFileDataSource(dir)
	result, err := ds.Scores("nonexistent-pair")
	if err != nil {
		t.Fatalf("Scores() error: %v", err)
	}
	entries, ok := result.([]parser.ScoreEntry)
	if !ok {
		t.Fatalf("expected []parser.ScoreEntry, got %T", result)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for nonexistent pair, got %d", len(entries))
	}
}

// --- M5 Hardening: Error-path tests (TDD — these should FAIL until graceful degradation is implemented) ---

// TestFileDataSource_Pairs_MissingWorkflow verifies that Pairs() returns an
// empty slice (not an error) when workflow.yaml does not exist.
func TestFileDataSource_Pairs_MissingWorkflow(t *testing.T) {
	dir := t.TempDir() // no workflow.yaml
	ds := NewFileDataSource(dir)

	result, err := ds.Pairs()
	if err != nil {
		t.Fatalf("Pairs() should return empty slice for missing workflow.yaml, got error: %v", err)
	}
	pairs, ok := result.([]PairStatus)
	if !ok {
		t.Fatalf("expected []PairStatus, got %T", result)
	}
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs for missing workflow.yaml, got %d", len(pairs))
	}
}

// TestFileDataSource_Pairs_MalformedWorkflow verifies that Pairs() returns an
// empty slice (not an error) when workflow.yaml contains invalid YAML.
func TestFileDataSource_Pairs_MalformedWorkflow(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte("{{{{not yaml"), 0o644)
	ds := NewFileDataSource(dir)

	result, err := ds.Pairs()
	if err != nil {
		t.Fatalf("Pairs() should return empty slice for malformed workflow.yaml, got error: %v", err)
	}
	pairs, ok := result.([]PairStatus)
	if !ok {
		t.Fatalf("expected []PairStatus, got %T", result)
	}
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs for malformed workflow.yaml, got %d", len(pairs))
	}
}

// TestFileDataSource_Pairs_PermissionDenied verifies that Pairs() still returns
// an error when workflow.yaml exists but is unreadable (permission denied).
// Only os.ErrNotExist and parse errors get the graceful-empty treatment.
func TestFileDataSource_Pairs_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yaml")
	os.WriteFile(path, []byte("version: 2\n"), 0o000)
	ds := NewFileDataSource(dir)

	_, err := ds.Pairs()
	if err == nil {
		t.Fatal("Pairs() should return an error for permission-denied workflow.yaml")
	}
}

// TestFileDataSource_Plan_MissingFile verifies that Plan() returns a zero-value
// PlanResponse (not an error) when plan.yaml does not exist.
func TestFileDataSource_Plan_MissingFile(t *testing.T) {
	dir := t.TempDir() // no plan.yaml, no workflow.yaml
	ds := NewFileDataSource(dir)

	result, err := ds.Plan()
	if err != nil {
		t.Fatalf("Plan() should return zero-value PlanResponse for missing plan.yaml, got error: %v", err)
	}
	resp, ok := result.(*PlanResponse)
	if !ok {
		t.Fatalf("expected *PlanResponse, got %T", result)
	}
	if resp.Epic.Name != "" {
		t.Errorf("expected empty epic name for missing plan.yaml, got %q", resp.Epic.Name)
	}
	if resp.MaxRegressions != 2 {
		t.Errorf("MaxRegressions: got %d, want 2 (default)", resp.MaxRegressions)
	}
}

// TestFileDataSource_Status_MissingPlan verifies that Status() returns a
// zero-value StatusInfo (not an error) when plan.yaml does not exist.
func TestFileDataSource_Status_MissingPlan(t *testing.T) {
	dir := t.TempDir() // no plan.yaml
	ds := NewFileDataSource(dir)

	result, err := ds.Status()
	if err != nil {
		t.Fatalf("Status() should return zero-value StatusInfo for missing plan.yaml, got error: %v", err)
	}
	info, ok := result.(*StatusInfo)
	if !ok {
		t.Fatalf("expected *StatusInfo, got %T", result)
	}
	if info.MilestoneID != 0 {
		t.Errorf("expected MilestoneID 0 for missing plan, got %d", info.MilestoneID)
	}
	if info.Phase != "" {
		t.Errorf("expected empty phase for missing plan, got %q", info.Phase)
	}
}

// --- Workspaces ---

// TestFileDataSource_Workspaces verifies that Workspaces() reads workspace
// entries from workflow.yaml.
func TestFileDataSource_Workspaces(t *testing.T) {
	dir := t.TempDir()
	workflow := `version: 2
max_rounds: 3
escalation: human
progress:
  adapter: none
workspaces:
  - name: frontend
    path: /home/dev/frontend
  - name: backend
    path: /home/dev/backend
pairs: []
`
	os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(workflow), 0o644)
	ds := NewFileDataSource(dir)

	result, err := ds.Workspaces()
	if err != nil {
		t.Fatalf("Workspaces() error: %v", err)
	}

	workspaces, ok := result.([]WorkspaceInfo)
	if !ok {
		t.Fatalf("expected []WorkspaceInfo, got %T", result)
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if workspaces[0].Name != "frontend" {
		t.Errorf("first workspace name: got %q, want %q", workspaces[0].Name, "frontend")
	}
	if workspaces[0].Path != "/home/dev/frontend" {
		t.Errorf("first workspace path: got %q, want %q", workspaces[0].Path, "/home/dev/frontend")
	}
	if workspaces[1].Name != "backend" {
		t.Errorf("second workspace name: got %q, want %q", workspaces[1].Name, "backend")
	}
}

// TestFileDataSource_Workspaces_MissingWorkflow verifies that Workspaces()
// returns an empty slice when workflow.yaml is missing.
func TestFileDataSource_Workspaces_MissingWorkflow(t *testing.T) {
	dir := t.TempDir()
	ds := NewFileDataSource(dir)

	result, err := ds.Workspaces()
	if err != nil {
		t.Fatalf("Workspaces() should not error for missing workflow.yaml: %v", err)
	}
	workspaces, ok := result.([]WorkspaceInfo)
	if !ok {
		t.Fatalf("expected []WorkspaceInfo, got %T", result)
	}
	if len(workspaces) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(workspaces))
	}
}

// TestFileDataSource_Workspaces_NoWorkspacesKey verifies that Workspaces()
// returns an empty slice when workflow.yaml has no workspaces key.
func TestFileDataSource_Workspaces_NoWorkspacesKey(t *testing.T) {
	dir := t.TempDir()
	workflow := `version: 2
max_rounds: 3
escalation: human
progress:
  adapter: none
pairs: []
`
	os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(workflow), 0o644)
	ds := NewFileDataSource(dir)

	result, err := ds.Workspaces()
	if err != nil {
		t.Fatalf("Workspaces() error: %v", err)
	}
	workspaces, ok := result.([]WorkspaceInfo)
	if !ok {
		t.Fatalf("expected []WorkspaceInfo, got %T", result)
	}
	if len(workspaces) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(workspaces))
	}
}

// TestFileDataSource_Workspaces_MalformedWorkflow verifies graceful degradation.
func TestFileDataSource_Workspaces_MalformedWorkflow(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte("{{{{not yaml"), 0o644)
	ds := NewFileDataSource(dir)

	result, err := ds.Workspaces()
	if err != nil {
		t.Fatalf("Workspaces() should not error for malformed workflow.yaml: %v", err)
	}
	workspaces, ok := result.([]WorkspaceInfo)
	if !ok {
		t.Fatalf("expected []WorkspaceInfo, got %T", result)
	}
	if len(workspaces) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(workspaces))
	}
}

// TestFileDataSource_Workspaces_PermissionDenied verifies real I/O errors propagate.
func TestFileDataSource_Workspaces_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yaml")
	os.WriteFile(path, []byte("version: 2\n"), 0o000)
	ds := NewFileDataSource(dir)

	_, err := ds.Workspaces()
	if err == nil {
		t.Fatal("Workspaces() should return an error for permission-denied workflow.yaml")
	}
}

// TestFileDataSource_Status_NilCurrentFocus verifies that Status() handles a
// plan with nil current_focus without error, returning zero-value StatusInfo.
func TestFileDataSource_Status_NilCurrentFocus(t *testing.T) {
	dir := t.TempDir()
	plan := `epic:
  name: test
  description: test plan
  milestones:
    - id: 1
      name: "M1"
      description: "desc"
      pairs: ["p1"]
      status: done
      done_when: "done"
`
	os.WriteFile(filepath.Join(dir, "plan.yaml"), []byte(plan), 0o644)
	ds := NewFileDataSource(dir)

	result, err := ds.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	info, ok := result.(*StatusInfo)
	if !ok {
		t.Fatalf("expected *StatusInfo, got %T", result)
	}
	if info.MilestoneID != 0 {
		t.Errorf("expected MilestoneID 0 with nil current_focus, got %d", info.MilestoneID)
	}
}

// TestFileDataSource_Debate_MalformedMeta verifies that Debate() returns a
// meaningful error when meta.json is malformed JSON.
func TestFileDataSource_Debate_MalformedMeta(t *testing.T) {
	dir := t.TempDir()
	debateDir := filepath.Join(dir, "debates", "bad-debate")
	os.MkdirAll(debateDir, 0o755)
	os.WriteFile(filepath.Join(debateDir, "meta.json"), []byte("{not valid json"), 0o644)
	ds := NewFileDataSource(dir)

	_, err := ds.Debate("bad-debate")
	if err == nil {
		t.Fatal("Debate() should return error for malformed meta.json")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should mention 'parse', got: %v", err)
	}
}

func TestFileDataSource_Debates_SkipsMalformed(t *testing.T) {
	dir := t.TempDir()

	// Create a malformed meta.json
	debateDir := filepath.Join(dir, "debates", "bad-debate")
	os.MkdirAll(debateDir, 0o755)
	os.WriteFile(filepath.Join(debateDir, "meta.json"), []byte("{bad json"), 0o644)

	// Create a valid meta.json
	goodDir := filepath.Join(dir, "debates", "good-debate")
	os.MkdirAll(goodDir, 0o755)
	meta := map[string]any{
		"id":          "good-debate",
		"pair":        "test",
		"phase":       "review",
		"milestone":   1,
		"files":       []string{},
		"status":      "initiated",
		"round_count": 0,
		"max_rounds":  3,
		"started":     time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(goodDir, "meta.json"), data, 0o644)

	ds := NewFileDataSource(dir)
	result, err := ds.Debates()
	if err != nil {
		t.Fatalf("Debates() should not fail with malformed files: %v", err)
	}

	debates, ok := result.([]parser.DebateMeta)
	if !ok {
		t.Fatalf("expected []parser.DebateMeta, got %T", result)
	}
	if len(debates) != 1 {
		t.Errorf("expected 1 debate (skipping malformed), got %d", len(debates))
	}
}

// --- V2 Tests: verify datasource exposes v2 plan structures ---

// TestFileDataSource_Plan_V2WithIssues verifies that Plan() correctly returns
// v2 plan.yaml structure with issues, depends_on, and regressions fields.
func TestFileDataSource_Plan_V2WithIssues(t *testing.T) {
	dir := t.TempDir()

	// Create a v2 plan.yaml with issues.
	planV2 := `epic:
  name: test-epic
  description: Testing v2 plan structure
  milestones:
    - id: 1
      name: "Milestone 1"
      description: "First milestone"
      status: done
      done_when: "all tests pass"
      depends_on: []
      regressions: 0
      issues:
        - ref: "issue-1-1"
          title: "First issue"
          pairs: ["test-pair"]
          depends_on: []
          phase_status:
            plan: done
            test: done
            build: done
          files: ["file1.go"]
          debates: ["debate-1"]
          branch: "feature/issue-1-1"
          status: done
    - id: 2
      name: "Milestone 2"
      description: "Second milestone"
      status: in_progress
      done_when: "integration complete"
      depends_on: [1]
      regressions: 1
      issues:
        - ref: "issue-2-1"
          title: "Second issue"
          pairs: ["api-contracts"]
          depends_on: []
          phase_status:
            plan: done
            test: in_progress
          files: []
          debates: []
          branch: ""
          status: in_progress
        - ref: "issue-2-2"
          title: "Third issue"
          pairs: ["api-contracts"]
          depends_on: ["issue-2-1"]
          phase_status:
            plan: pending
          files: []
          debates: []
          branch: ""
          status: pending
  current_focus:
    milestone_id: 2
    issue_ref: "issue-2-1"
    phase: test
    started: "2026-03-16"
`
	os.WriteFile(filepath.Join(dir, "plan.yaml"), []byte(planV2), 0o644)

	ds := NewFileDataSource(dir)
	result, err := ds.Plan()
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	resp, ok := result.(*PlanResponse)
	if !ok {
		t.Fatalf("expected *PlanResponse, got %T", result)
	}

	// No workflow.yaml in this dir → defaults to 2.
	if resp.MaxRegressions != 2 {
		t.Errorf("MaxRegressions: got %d, want 2 (default)", resp.MaxRegressions)
	}

	plan := resp.Plan

	// Verify v2 milestone fields.
	if len(plan.Epic.Milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(plan.Epic.Milestones))
	}

	m1 := plan.Epic.Milestones[0]
	if len(m1.DependsOn) != 0 {
		t.Errorf("milestone 1 depends_on: got %v, want []", m1.DependsOn)
	}
	if m1.Regressions != 0 {
		t.Errorf("milestone 1 regressions: got %d, want 0", m1.Regressions)
	}
	if len(m1.Issues) != 1 {
		t.Fatalf("milestone 1 issues: got %d, want 1", len(m1.Issues))
	}
	if m1.Issues[0].Ref != "issue-1-1" {
		t.Errorf("milestone 1 issue ref: got %q, want %q", m1.Issues[0].Ref, "issue-1-1")
	}

	m2 := plan.Epic.Milestones[1]
	if len(m2.DependsOn) != 1 || m2.DependsOn[0] != 1 {
		t.Errorf("milestone 2 depends_on: got %v, want [1]", m2.DependsOn)
	}
	if m2.Regressions != 1 {
		t.Errorf("milestone 2 regressions: got %d, want 1", m2.Regressions)
	}
	if len(m2.Issues) != 2 {
		t.Fatalf("milestone 2 issues: got %d, want 2", len(m2.Issues))
	}
	if m2.Issues[1].Ref != "issue-2-2" {
		t.Errorf("milestone 2 second issue ref: got %q, want %q", m2.Issues[1].Ref, "issue-2-2")
	}
	if len(m2.Issues[1].DependsOn) != 1 || m2.Issues[1].DependsOn[0] != "issue-2-1" {
		t.Errorf("issue-2-2 depends_on: got %v, want [issue-2-1]", m2.Issues[1].DependsOn)
	}
}

// TestFileDataSource_Plan_MaxRegressionsFromWorkflow verifies that Plan()
// reads max_regressions from workflow.yaml and includes it in PlanResponse.
func TestFileDataSource_Plan_MaxRegressionsFromWorkflow(t *testing.T) {
	dir := t.TempDir()

	workflow := `version: 2
max_rounds: 3
escalation: human
max_regressions: 5
`
	os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(workflow), 0o644)

	plan := `epic:
  name: test-epic
  milestones:
    - id: 1
      name: M1
      status: in_progress
      regressions: 3
`
	os.WriteFile(filepath.Join(dir, "plan.yaml"), []byte(plan), 0o644)

	ds := NewFileDataSource(dir)
	result, err := ds.Plan()
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	resp, ok := result.(*PlanResponse)
	if !ok {
		t.Fatalf("expected *PlanResponse, got %T", result)
	}
	if resp.MaxRegressions != 5 {
		t.Errorf("MaxRegressions: got %d, want 5", resp.MaxRegressions)
	}
	if resp.Epic.Milestones[0].Regressions != 3 {
		t.Errorf("milestone regressions: got %d, want 3", resp.Epic.Milestones[0].Regressions)
	}
}

// TestFileDataSource_Status_V2WithIssue verifies that Status() includes
// the current issue reference from v2 plan.yaml current_focus.
func TestFileDataSource_Status_V2WithIssue(t *testing.T) {
	dir := t.TempDir()

	// Create a v2 plan.yaml with issue reference in current_focus.
	planV2 := `epic:
  name: test-epic
  description: Testing v2 status
  milestones:
    - id: 1
      name: "API v2"
      description: "Update to v2"
      status: in_progress
      done_when: "done"
      depends_on: []
      regressions: 0
      issues:
        - ref: "issue-1-1"
          title: "Datasource v2"
          pairs: ["api-contracts"]
          depends_on: []
          phase_status:
            plan: done
            test: in_progress
          files: []
          debates: []
          branch: ""
          status: in_progress
  current_focus:
    milestone_id: 1
    issue_ref: "issue-1-1"
    phase: test
    started: "2026-03-16"
`
	os.WriteFile(filepath.Join(dir, "plan.yaml"), []byte(planV2), 0o644)

	ds := NewFileDataSource(dir)
	result, err := ds.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	info, ok := result.(*StatusInfo)
	if !ok {
		t.Fatalf("expected *StatusInfo, got %T", result)
	}

	if info.MilestoneID != 1 {
		t.Errorf("MilestoneID: got %d, want 1", info.MilestoneID)
	}
	if info.MilestoneName != "API v2" {
		t.Errorf("MilestoneName: got %q, want %q", info.MilestoneName, "API v2")
	}
	if info.Phase != "test" {
		t.Errorf("Phase: got %q, want %q", info.Phase, "test")
	}
	// V2 field: issue reference.
	if info.IssueRef != "issue-1-1" {
		t.Errorf("IssueRef: got %q, want %q", info.IssueRef, "issue-1-1")
	}
}
