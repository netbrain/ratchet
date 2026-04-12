package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/classifier"
	"github.com/netbrain/ratchet-monitor/internal/events"
	"github.com/netbrain/ratchet-monitor/internal/parser"
	"github.com/netbrain/ratchet-monitor/internal/sse"
	"github.com/netbrain/ratchet-monitor/internal/watcher"
)

// newTestPipeline creates a Pipeline with a real classifier and the given
// rootDir, but nil watcher (only used for enrich/parseFile tests that
// don't call Run).
func newTestPipeline(rootDir string) *Pipeline {
	return &Pipeline{
		rootDir:    rootDir,
		classifier: classifier.New(),
	}
}

// --- enrich tests ---

func TestEnrich_DebateMeta_Initiated(t *testing.T) {
	dir := t.TempDir()
	debateDir := filepath.Join(dir, "debates", "test-debate-123")
	if err := os.MkdirAll(debateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	meta := parser.DebateMeta{
		ID:     "test-debate-issue42-20260317T083800",
		Pair:   "test-pair",
		Phase:  "plan",
		Status: "initiated",
	}
	data, _ := json.Marshal(meta)
	metaPath := filepath.Join(debateDir, "meta.json")
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   1,
		Type: events.FileCreated,
		Path: metaPath,
	}

	got := p.enrich(ev)

	if got.Type != events.DebateStarted {
		t.Errorf("Type = %q, want %q", got.Type, events.DebateStarted)
	}
	if got.Issue != "issue42" {
		t.Errorf("Issue = %q, want %q", got.Issue, "issue42")
	}
	if got.Data == nil {
		t.Error("Data should not be nil for a valid debate meta")
	}
}

func TestEnrich_DebateMeta_Consensus(t *testing.T) {
	dir := t.TempDir()
	debateDir := filepath.Join(dir, "debates", "resolved-debate")
	if err := os.MkdirAll(debateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	meta := parser.DebateMeta{
		ID:     "resolved-debate",
		Status: "consensus",
	}
	data, _ := json.Marshal(meta)
	metaPath := filepath.Join(debateDir, "meta.json")
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   2,
		Type: events.FileModified,
		Path: metaPath,
	}

	got := p.enrich(ev)

	if got.Type != events.DebateResolved {
		t.Errorf("Type = %q, want %q", got.Type, events.DebateResolved)
	}
}

func TestEnrich_DebateMeta_Escalated(t *testing.T) {
	dir := t.TempDir()
	debateDir := filepath.Join(dir, "debates", "escalated-debate")
	if err := os.MkdirAll(debateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	meta := parser.DebateMeta{
		ID:     "escalated-debate",
		Status: "escalated",
	}
	data, _ := json.Marshal(meta)
	metaPath := filepath.Join(debateDir, "meta.json")
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   3,
		Type: events.FileModified,
		Path: metaPath,
	}

	got := p.enrich(ev)

	if got.Type != events.DebateResolved {
		t.Errorf("Type = %q, want %q", got.Type, events.DebateResolved)
	}
}

func TestEnrich_DebateMeta_WithIssueRef(t *testing.T) {
	dir := t.TempDir()
	debateDir := filepath.Join(dir, "debates", "ref-debate")
	if err := os.MkdirAll(debateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	meta := parser.DebateMeta{
		ID:       "ref-debate",
		IssueRef: "#99",
		Status:   "initiated",
	}
	data, _ := json.Marshal(meta)
	metaPath := filepath.Join(debateDir, "meta.json")
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   4,
		Type: events.FileCreated,
		Path: metaPath,
	}

	got := p.enrich(ev)

	if got.Issue != "#99" {
		t.Errorf("Issue = %q, want %q", got.Issue, "#99")
	}
}

func TestEnrich_DebateMeta_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	debateDir := filepath.Join(dir, "debates", "bad-debate")
	if err := os.MkdirAll(debateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	metaPath := filepath.Join(debateDir, "meta.json")
	if err := os.WriteFile(metaPath, []byte("{broken json"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   5,
		Type: events.FileModified,
		Path: metaPath,
	}

	got := p.enrich(ev)

	// With malformed JSON, parseFile returns nil, so classifier falls back
	// to pattern-based classification. For a modified debates/*/meta.json
	// with nil content, it should return DebateUpdated.
	if got.Type != events.DebateUpdated {
		t.Errorf("Type = %q, want %q", got.Type, events.DebateUpdated)
	}
	if got.Data != nil {
		t.Error("Data should be nil for malformed JSON")
	}
}

func TestEnrich_ScoresFile(t *testing.T) {
	dir := t.TempDir()
	scoresDir := filepath.Join(dir, "scores")
	if err := os.MkdirAll(scoresDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entry := parser.ScoreEntry{
		Timestamp: time.Date(2026, 3, 17, 8, 0, 0, 0, time.UTC),
		DebateID:  "debate-1",
		Pair:      "pair-1",
		Milestone: 1,
	}
	data, _ := json.Marshal(entry)
	scoresPath := filepath.Join(scoresDir, "scores.jsonl")
	if err := os.WriteFile(scoresPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   6,
		Type: events.FileModified,
		Path: scoresPath,
	}

	got := p.enrich(ev)

	if got.Type != events.ScoreUpdated {
		t.Errorf("Type = %q, want %q", got.Type, events.ScoreUpdated)
	}
	if got.Data == nil {
		t.Error("Data should not be nil for valid scores file")
	}
}

func TestEnrich_ScoresFile_MalformedLines(t *testing.T) {
	dir := t.TempDir()
	scoresDir := filepath.Join(dir, "scores")
	if err := os.MkdirAll(scoresDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// One valid line, one malformed
	content := `{"timestamp":"2026-03-17T08:00:00Z","debate_id":"d1","pair":"p1","milestone":1}
{broken line}`
	scoresPath := filepath.Join(scoresDir, "scores.jsonl")
	if err := os.WriteFile(scoresPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   7,
		Type: events.FileModified,
		Path: scoresPath,
	}

	got := p.enrich(ev)

	if got.Type != events.ScoreUpdated {
		t.Errorf("Type = %q, want %q", got.Type, events.ScoreUpdated)
	}
	entries, ok := got.Data.([]parser.ScoreEntry)
	if !ok {
		t.Fatalf("Data is %T, want []parser.ScoreEntry", got.Data)
	}
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1 (malformed line should be skipped)", len(entries))
	}
}

func TestEnrich_PlanFile(t *testing.T) {
	dir := t.TempDir()

	planContent := `epic:
  name: "Test Epic"
  description: "A test"
  milestones:
    - id: 1
      name: "M1"
      status: "active"
`
	planPath := filepath.Join(dir, "plan.yaml")
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   8,
		Type: events.FileModified,
		Path: planPath,
	}

	got := p.enrich(ev)

	if got.Type != events.PlanUpdated {
		t.Errorf("Type = %q, want %q", got.Type, events.PlanUpdated)
	}
	plan, ok := got.Data.(*parser.Plan)
	if !ok {
		t.Fatalf("Data is %T, want *parser.Plan", got.Data)
	}
	if plan.Epic.Name != "Test Epic" {
		t.Errorf("Epic.Name = %q, want %q", plan.Epic.Name, "Test Epic")
	}
}

func TestEnrich_PlanFile_MalformedYAML(t *testing.T) {
	dir := t.TempDir()

	planPath := filepath.Join(dir, "plan.yaml")
	if err := os.WriteFile(planPath, []byte(":\n  :\n  bad: [yaml: }{"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   9,
		Type: events.FileModified,
		Path: planPath,
	}

	got := p.enrich(ev)

	if got.Type != events.PlanUpdated {
		t.Errorf("Type = %q, want %q", got.Type, events.PlanUpdated)
	}
	if got.Data != nil {
		t.Error("Data should be nil for malformed YAML")
	}
}

func TestEnrich_WorkflowFile(t *testing.T) {
	dir := t.TempDir()

	wfContent := `version: 1
max_rounds: 3
escalation: "human"
`
	wfPath := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(wfPath, []byte(wfContent), 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   10,
		Type: events.FileModified,
		Path: wfPath,
	}

	got := p.enrich(ev)

	if got.Type != events.ConfigChanged {
		t.Errorf("Type = %q, want %q", got.Type, events.ConfigChanged)
	}
	wf, ok := got.Data.(*parser.WorkflowConfig)
	if !ok {
		t.Fatalf("Data is %T, want *parser.WorkflowConfig", got.Data)
	}
	if wf.MaxRounds != 3 {
		t.Errorf("MaxRounds = %d, want 3", wf.MaxRounds)
	}
}

func TestEnrich_WorkflowFile_MalformedYAML(t *testing.T) {
	dir := t.TempDir()

	wfPath := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(wfPath, []byte(":\n  bad: [yaml: }{"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   11,
		Type: events.FileModified,
		Path: wfPath,
	}

	got := p.enrich(ev)

	if got.Type != events.ConfigChanged {
		t.Errorf("Type = %q, want %q", got.Type, events.ConfigChanged)
	}
	if got.Data != nil {
		t.Error("Data should be nil for malformed workflow YAML")
	}
}

func TestEnrich_ProjectFile(t *testing.T) {
	dir := t.TempDir()

	projContent := `name: "Test Project"
description: "A test project"
stack:
  language: "Go"
  version: "1.23"
`
	projPath := filepath.Join(dir, "project.yaml")
	if err := os.WriteFile(projPath, []byte(projContent), 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   12,
		Type: events.FileModified,
		Path: projPath,
	}

	got := p.enrich(ev)

	if got.Type != events.ConfigChanged {
		t.Errorf("Type = %q, want %q", got.Type, events.ConfigChanged)
	}
	proj, ok := got.Data.(*parser.ProjectConfig)
	if !ok {
		t.Fatalf("Data is %T, want *parser.ProjectConfig", got.Data)
	}
	if proj.Name != "Test Project" {
		t.Errorf("Name = %q, want %q", proj.Name, "Test Project")
	}
}

func TestEnrich_ProjectFile_MalformedYAML(t *testing.T) {
	dir := t.TempDir()

	projPath := filepath.Join(dir, "project.yaml")
	if err := os.WriteFile(projPath, []byte(":\n  bad: [yaml: }{"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   13,
		Type: events.FileModified,
		Path: projPath,
	}

	got := p.enrich(ev)

	if got.Type != events.ConfigChanged {
		t.Errorf("Type = %q, want %q", got.Type, events.ConfigChanged)
	}
	if got.Data != nil {
		t.Error("Data should be nil for malformed project YAML")
	}
}

func TestEnrich_FileDeleted(t *testing.T) {
	dir := t.TempDir()

	// Deleted files should NOT be read; they get classified by path only.
	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   14,
		Type: events.FileDeleted,
		Path: filepath.Join(dir, "plan.yaml"),
	}

	got := p.enrich(ev)

	if got.Type != events.PlanUpdated {
		t.Errorf("Type = %q, want %q", got.Type, events.PlanUpdated)
	}
	if got.Data != nil {
		t.Error("Data should be nil for deleted file")
	}
}

func TestEnrich_FileDeleted_DebateMeta(t *testing.T) {
	dir := t.TempDir()

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   15,
		Type: events.FileDeleted,
		Path: filepath.Join(dir, "debates", "some-debate", "meta.json"),
	}

	got := p.enrich(ev)

	// Deleted meta.json (not FileCreated) -> classifier returns DebateUpdated.
	if got.Type != events.DebateUpdated {
		t.Errorf("Type = %q, want %q", got.Type, events.DebateUpdated)
	}
}

func TestEnrich_UnreadableFile(t *testing.T) {
	dir := t.TempDir()

	// Reference a file that does not exist on disk.
	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   16,
		Type: events.FileModified,
		Path: filepath.Join(dir, "plan.yaml"), // does not exist
	}

	got := p.enrich(ev)

	// Can't read -> nil content -> classifier sees plan.yaml path -> PlanUpdated.
	if got.Type != events.PlanUpdated {
		t.Errorf("Type = %q, want %q", got.Type, events.PlanUpdated)
	}
	if got.Data != nil {
		t.Error("Data should be nil when file cannot be read")
	}
}

func TestEnrich_UnrecognizedFile(t *testing.T) {
	dir := t.TempDir()

	unknownPath := filepath.Join(dir, "unknown.txt")
	if err := os.WriteFile(unknownPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   17,
		Type: events.FileModified,
		Path: unknownPath,
	}

	got := p.enrich(ev)

	// Unrecognized file -> parseFile returns nil -> classifier falls back
	// to the raw event type.
	if got.Type != events.FileModified {
		t.Errorf("Type = %q, want %q", got.Type, events.FileModified)
	}
	if got.Data != nil {
		t.Error("Data should be nil for unrecognized file type")
	}
}

func TestEnrich_WorkspacePath(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "workspaces", "monitor", ".ratchet")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	planContent := `epic:
  name: "Workspace Plan"
`
	planPath := filepath.Join(wsDir, "plan.yaml")
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatal(err)
	}

	p := newTestPipeline(dir)
	ev := events.Event{
		ID:   18,
		Type: events.FileModified,
		Path: planPath,
	}

	got := p.enrich(ev)

	if got.Workspace != "monitor" {
		t.Errorf("Workspace = %q, want %q", got.Workspace, "monitor")
	}
	if got.Type != events.PlanUpdated {
		t.Errorf("Type = %q, want %q", got.Type, events.PlanUpdated)
	}
}

func TestEnrich_EmptyRootDir(t *testing.T) {
	dir := t.TempDir()

	planContent := `epic:
  name: "Plan"
`
	planPath := filepath.Join(dir, "plan.yaml")
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// When rootDir is empty, relPath stays as the absolute ev.Path.
	p := &Pipeline{
		rootDir:    "",
		classifier: classifier.New(),
	}
	ev := events.Event{
		ID:   19,
		Type: events.FileModified,
		Path: planPath,
	}

	got := p.enrich(ev)

	// The absolute path ends with /plan.yaml so classifier should still
	// match the suffix pattern.
	if got.Type != events.PlanUpdated {
		t.Errorf("Type = %q, want %q", got.Type, events.PlanUpdated)
	}
}

// --- parseFile tests ---

func TestParseFile_DebateMeta_Valid(t *testing.T) {
	p := newTestPipeline("")
	meta := parser.DebateMeta{ID: "test-id", Status: "initiated"}
	data, _ := json.Marshal(meta)

	result := p.parseFile("debates/test/meta.json", data)
	if result == nil {
		t.Fatal("expected non-nil result for valid debate meta")
	}
	dm, ok := result.(*parser.DebateMeta)
	if !ok {
		t.Fatalf("result is %T, want *parser.DebateMeta", result)
	}
	if dm.ID != "test-id" {
		t.Errorf("ID = %q, want %q", dm.ID, "test-id")
	}
}

func TestParseFile_DebateMeta_Empty(t *testing.T) {
	p := newTestPipeline("")
	result := p.parseFile("debates/test/meta.json", []byte{})
	if result != nil {
		t.Error("expected nil for empty debate meta data")
	}
}

func TestParseFile_DebateMeta_Invalid(t *testing.T) {
	p := newTestPipeline("")
	result := p.parseFile("debates/test/meta.json", []byte("{bad"))
	if result != nil {
		t.Error("expected nil for invalid debate meta JSON")
	}
}

func TestParseFile_Scores_Valid(t *testing.T) {
	p := newTestPipeline("")
	content := `{"timestamp":"2026-03-17T08:00:00Z","debate_id":"d1","pair":"p1","milestone":1}`
	result := p.parseFile("scores/scores.jsonl", []byte(content))
	if result == nil {
		t.Fatal("expected non-nil result for valid scores")
	}
	entries, ok := result.([]parser.ScoreEntry)
	if !ok {
		t.Fatalf("result is %T, want []parser.ScoreEntry", result)
	}
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1", len(entries))
	}
}

func TestParseFile_Plan_Valid(t *testing.T) {
	p := newTestPipeline("")
	content := `epic:
  name: "Test"
`
	result := p.parseFile("plan.yaml", []byte(content))
	if result == nil {
		t.Fatal("expected non-nil result for valid plan")
	}
	plan, ok := result.(*parser.Plan)
	if !ok {
		t.Fatalf("result is %T, want *parser.Plan", result)
	}
	if plan.Epic.Name != "Test" {
		t.Errorf("Name = %q, want %q", plan.Epic.Name, "Test")
	}
}

func TestParseFile_Plan_Invalid(t *testing.T) {
	p := newTestPipeline("")
	result := p.parseFile("plan.yaml", []byte(":\n  bad: [yaml: }{"))
	if result != nil {
		t.Error("expected nil for invalid plan YAML")
	}
}

func TestParseFile_Workflow_Valid(t *testing.T) {
	p := newTestPipeline("")
	content := `version: 1
max_rounds: 5
escalation: "human"
`
	result := p.parseFile("workflow.yaml", []byte(content))
	if result == nil {
		t.Fatal("expected non-nil result for valid workflow")
	}
	wf, ok := result.(*parser.WorkflowConfig)
	if !ok {
		t.Fatalf("result is %T, want *parser.WorkflowConfig", result)
	}
	if wf.MaxRounds != 5 {
		t.Errorf("MaxRounds = %d, want 5", wf.MaxRounds)
	}
}

func TestParseFile_Workflow_Invalid(t *testing.T) {
	p := newTestPipeline("")
	result := p.parseFile("workflow.yaml", []byte(":\n  bad: [yaml: }{"))
	if result != nil {
		t.Error("expected nil for invalid workflow YAML")
	}
}

func TestParseFile_Project_Valid(t *testing.T) {
	p := newTestPipeline("")
	content := `name: "proj"
description: "desc"
`
	result := p.parseFile("project.yaml", []byte(content))
	if result == nil {
		t.Fatal("expected non-nil result for valid project")
	}
	proj, ok := result.(*parser.ProjectConfig)
	if !ok {
		t.Fatalf("result is %T, want *parser.ProjectConfig", result)
	}
	if proj.Name != "proj" {
		t.Errorf("Name = %q, want %q", proj.Name, "proj")
	}
}

func TestParseFile_Project_Invalid(t *testing.T) {
	p := newTestPipeline("")
	result := p.parseFile("project.yaml", []byte(":\n  bad: [yaml: }{"))
	if result != nil {
		t.Error("expected nil for invalid project YAML")
	}
}

func TestParseFile_Unknown(t *testing.T) {
	p := newTestPipeline("")
	result := p.parseFile("unknown.txt", []byte("hello world"))
	if result != nil {
		t.Error("expected nil for unknown file type")
	}
}

func TestParseFile_NestedPlan(t *testing.T) {
	p := newTestPipeline("")
	content := `epic:
  name: "Nested"
`
	result := p.parseFile("workspaces/monitor/.ratchet/plan.yaml", []byte(content))
	if result == nil {
		t.Fatal("expected non-nil result for nested plan.yaml")
	}
}

func TestParseFile_NestedWorkflow(t *testing.T) {
	p := newTestPipeline("")
	content := `version: 1
max_rounds: 3
escalation: "human"
`
	result := p.parseFile("workspaces/api/.ratchet/workflow.yaml", []byte(content))
	if result == nil {
		t.Fatal("expected non-nil result for nested workflow.yaml")
	}
}

func TestParseFile_NestedProject(t *testing.T) {
	p := newTestPipeline("")
	content := `name: "nested"
description: "nested project"
`
	result := p.parseFile("workspaces/api/.ratchet/project.yaml", []byte(content))
	if result == nil {
		t.Fatal("expected non-nil result for nested project.yaml")
	}
}

// --- New constructor ---

func TestNew(t *testing.T) {
	dir := t.TempDir()
	w, err := watcher.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	b := sse.NewBroker()
	defer b.Close()

	p := New(w, b, dir)

	if p.watcher != w {
		t.Error("watcher not set correctly")
	}
	if p.broker != b {
		t.Error("broker not set correctly")
	}
	if p.rootDir != dir {
		t.Errorf("rootDir = %q, want %q", p.rootDir, dir)
	}
	if p.classifier == nil {
		t.Error("classifier should not be nil")
	}
}

// --- Run integration tests ---

func TestRun_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	w, err := watcher.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	b := sse.NewBroker()
	defer b.Close()

	p := New(w, b, dir)

	ctx, cancel := context.WithCancel(context.Background())

	// Start the watcher event loop.
	go w.Run(ctx)

	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	// Cancel should cause Run to exit.
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

func TestRun_ProcessesEvent(t *testing.T) {
	dir := t.TempDir()

	// Pre-create the file so modification events are detected.
	planPath := filepath.Join(dir, "plan.yaml")
	if err := os.WriteFile(planPath, []byte("epic:\n  name: Initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := watcher.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	b := sse.NewBroker()
	defer b.Close()

	sub, err := b.Subscribe()
	if err != nil {
		t.Fatal(err)
	}

	p := New(w, b, dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)
	go p.Run(ctx)

	// Give watcher time to start.
	time.Sleep(100 * time.Millisecond)

	// Modify the file to trigger an event.
	if err := os.WriteFile(planPath, []byte("epic:\n  name: Updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for the enriched event to arrive via SSE broker.
	select {
	case ev := <-sub.Events():
		if ev.Type != events.PlanUpdated {
			t.Errorf("Type = %q, want %q", ev.Type, events.PlanUpdated)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for enriched event from pipeline")
	}
}

func TestRun_ProcessesDebateMetaEvent(t *testing.T) {
	dir := t.TempDir()
	debateDir := filepath.Join(dir, "debates", "run-test")
	if err := os.MkdirAll(debateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	w, err := watcher.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	b := sse.NewBroker()
	defer b.Close()

	sub, err := b.Subscribe()
	if err != nil {
		t.Fatal(err)
	}

	p := New(w, b, dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)
	go p.Run(ctx)

	time.Sleep(100 * time.Millisecond)

	// Create a meta.json to trigger FileCreated -> DebateStarted.
	meta := parser.DebateMeta{
		ID:     "run-test-issue7-20260317T120000",
		Status: "initiated",
	}
	data, _ := json.Marshal(meta)
	metaPath := filepath.Join(debateDir, "meta.json")
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case ev := <-sub.Events():
		if ev.Type != events.DebateStarted {
			t.Errorf("Type = %q, want %q", ev.Type, events.DebateStarted)
		}
		if ev.Issue != "issue7" {
			t.Errorf("Issue = %q, want %q", ev.Issue, "issue7")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for debate event")
	}
}
