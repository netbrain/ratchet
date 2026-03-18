// Package datasource provides read-only access to parsed .ratchet/ data.
package datasource

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/netbrain/ratchet-monitor/internal/handler"
	"github.com/netbrain/ratchet-monitor/internal/parser"
)

// FileDataSource reads and parses .ratchet/ files from disk.
// It implements handler.DataSource.
type FileDataSource struct {
	rootDir string
}

// NewFileDataSource creates a FileDataSource rooted at the given directory.
func NewFileDataSource(rootDir string) *FileDataSource {
	return &FileDataSource{rootDir: rootDir}
}

// WorkspaceInfo describes a workspace from workflow.yaml.
type WorkspaceInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// PairStatus summarizes a pair's current state derived from workflow config
// and active debates.
type PairStatus struct {
	Name      string `json:"name"`
	Component string `json:"component"`
	Phase     string `json:"phase"`
	Scope     string `json:"scope"`
	Enabled   bool   `json:"enabled"`
	Active    bool   `json:"active"`
	Status    string `json:"status"`
}

// loadWorkflowRaw reads workflow.yaml bytes.
// Returns (nil, nil) when the file does not exist (graceful missing).
// Real I/O errors (e.g., permission denied) are propagated.
func (f *FileDataSource) loadWorkflowRaw() ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(f.rootDir, "workflow.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workflow.yaml: %w", err)
	}
	return data, nil
}

// validateWorkspace checks that workspace is a known entry in the parsed config.
// Returns NotFoundError when not found.
func validateWorkspace(workspace string, wf *parser.WorkflowConfig) error {
	for _, ws := range wf.Workspaces {
		if ws.Name == workspace {
			return nil
		}
	}
	return &handler.NotFoundError{Resource: "workspace", ID: workspace}
}

// Pairs reads workflow.yaml and derives pair status from active debates.
// When workspace is non-empty, validates it against the workspace list and
// returns NotFoundError for unknown workspaces.
// Returns an empty slice when workflow.yaml is missing or contains invalid YAML.
// Real I/O errors (e.g., permission denied) are still propagated.
func (f *FileDataSource) Pairs(workspace string) (any, error) {
	data, err := f.loadWorkflowRaw()
	if err != nil {
		return nil, err
	}

	if data == nil {
		// workflow.yaml missing
		if workspace != "" {
			return nil, &handler.NotFoundError{Resource: "workspace", ID: workspace}
		}
		slog.Debug("workflow.yaml not found, returning empty pairs")
		return []PairStatus{}, nil
	}

	wf, err := parser.ParseWorkflow(data)
	if err != nil {
		if workspace != "" {
			// When a specific workspace is requested, we cannot validate it
			// against a malformed config. Return 404 — the caller asked for
			// a named workspace and we cannot confirm it exists.
			return nil, &handler.NotFoundError{Resource: "workspace", ID: workspace}
		}
		// Graceful degradation for the unfiltered case: treat unparseable YAML
		// the same as a missing file so the dashboard renders an empty state.
		slog.Warn("malformed workflow.yaml, returning empty pairs", "error", err)
		return []PairStatus{}, nil
	}

	if workspace != "" {
		if err := validateWorkspace(workspace, wf); err != nil {
			return nil, err
		}
	}

	// Build a map of pair name → derived status from debate metadata.
	statusMap := f.pairStatusMap()

	pairs := make([]PairStatus, 0, len(wf.Pairs))
	for _, p := range wf.Pairs {
		st := statusMap[p.Name]
		active := st == "debating"
		pairs = append(pairs, PairStatus{
			Name:      p.Name,
			Component: p.Component,
			Phase:     p.Phase,
			Scope:     p.Scope,
			Enabled:   p.Enabled,
			Active:    active,
			Status:    st,
		})
	}
	return pairs, nil
}

// Workspaces reads workflow.yaml and returns workspace entries.
// Returns an empty slice when workflow.yaml is missing or contains invalid YAML.
// Real I/O errors (e.g., permission denied) are still propagated.
func (f *FileDataSource) Workspaces() (any, error) {
	data, err := os.ReadFile(filepath.Join(f.rootDir, "workflow.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Debug("workflow.yaml not found, returning empty workspaces")
			return []WorkspaceInfo{}, nil
		}
		return nil, fmt.Errorf("read workflow.yaml: %w", err)
	}

	wf, err := parser.ParseWorkflow(data)
	if err != nil {
		slog.Warn("malformed workflow.yaml, returning empty workspaces", "error", err)
		return []WorkspaceInfo{}, nil
	}

	workspaces := make([]WorkspaceInfo, 0, len(wf.Workspaces))
	for _, ws := range wf.Workspaces {
		workspaces = append(workspaces, WorkspaceInfo{
			Name: ws.Name,
			Path: ws.Path,
		})
	}
	return workspaces, nil
}

// pairStatusMap scans debates/*/meta.json and derives a status string for
// each pair: "debating" if any active debate exists, "escalated" if the most
// recent debate is escalated, "consensus" if the most recent debate reached
// consensus, or "idle" otherwise.
func (f *FileDataSource) pairStatusMap() map[string]string {
	result := make(map[string]string)
	pattern := filepath.Join(f.rootDir, "debates", "*", "meta.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		slog.Debug("glob debates for pair status failed", "error", err)
		return result
	}

	// Track the most recent debate per pair and whether any is active.
	type pairInfo struct {
		hasActive  bool
		latestMeta *parser.DebateMeta
	}
	byPair := make(map[string]*pairInfo)

	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			slog.Debug("skip unreadable debate meta", "path", m, "error", err)
			continue
		}
		meta, err := parser.ParseDebateMeta(data)
		if err != nil {
			slog.Debug("skip malformed debate meta", "path", m, "error", err)
			continue
		}
		info, ok := byPair[meta.Pair]
		if !ok {
			info = &pairInfo{}
			byPair[meta.Pair] = info
		}
		if meta.Status != "consensus" && meta.Status != "escalated" {
			info.hasActive = true
		}
		if info.latestMeta == nil || meta.Started.After(info.latestMeta.Started) {
			info.latestMeta = meta
		}
	}

	for pair, info := range byPair {
		switch {
		case info.hasActive:
			result[pair] = "debating"
		case info.latestMeta != nil && info.latestMeta.Status == "escalated":
			result[pair] = "escalated"
		case info.latestMeta != nil && info.latestMeta.Status == "consensus":
			result[pair] = "consensus"
		default:
			result[pair] = "idle"
		}
	}
	return result
}

// Debates returns all debate metadata by globbing debates/*/meta.json.
// When workspace is non-empty, validates it against the workspace list and
// returns NotFoundError for unknown workspaces.
// Malformed or unreadable files are skipped.
func (f *FileDataSource) Debates(workspace string) (any, error) {
	if workspace != "" {
		data, err := f.loadWorkflowRaw()
		if err != nil {
			return nil, err
		}
		if data == nil {
			return nil, &handler.NotFoundError{Resource: "workspace", ID: workspace}
		}
		wf, err := parser.ParseWorkflow(data)
		if err != nil {
			return nil, &handler.NotFoundError{Resource: "workspace", ID: workspace}
		}
		if err := validateWorkspace(workspace, wf); err != nil {
			return nil, err
		}
	}

	pattern := filepath.Join(f.rootDir, "debates", "*", "meta.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob debates: %w", err)
	}

	debates := make([]parser.DebateMeta, 0)
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			slog.Debug("skip unreadable debate meta", "path", m, "error", err)
			continue
		}
		meta, err := parser.ParseDebateMeta(data)
		if err != nil {
			slog.Debug("skip malformed debate meta", "path", m, "error", err)
			continue
		}
		debates = append(debates, *meta)
	}

	// Sort by started time descending (most recent first).
	sort.Slice(debates, func(i, j int) bool {
		return debates[i].Started.After(debates[j].Started)
	})

	return debates, nil
}

// Debate reads a single debate's meta.json and its round files,
// returning a DebateWithRounds.
func (f *FileDataSource) Debate(id string) (any, error) {
	debateDir := filepath.Join(f.rootDir, "debates", id)
	metaPath := filepath.Join(debateDir, "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &handler.NotFoundError{Resource: "debate", ID: id}
		}
		return nil, fmt.Errorf("read debate %s: %w", id, err)
	}

	meta, err := parser.ParseDebateMeta(data)
	if err != nil {
		return nil, fmt.Errorf("parse debate %s: %w", id, err)
	}

	result := &parser.DebateWithRounds{
		DebateMeta: *meta,
		Rounds:     make([]parser.Round, 0),
	}

	// Read round files.
	roundsDir := filepath.Join(debateDir, "rounds")
	roundPattern := filepath.Join(roundsDir, "round-*.md")
	roundFiles, err := filepath.Glob(roundPattern)
	if err != nil {
		slog.Debug("glob rounds failed", "debate", id, "error", err)
		return result, nil
	}

	for _, rf := range roundFiles {
		name := filepath.Base(rf)
		num, role, err := parser.ParseRoundFilename(name)
		if err != nil {
			slog.Debug("skip malformed round file", "path", rf, "error", err)
			continue
		}
		content, err := os.ReadFile(rf)
		if err != nil {
			slog.Debug("skip unreadable round file", "path", rf, "error", err)
			continue
		}
		result.Rounds = append(result.Rounds, parser.Round{
			Number:  num,
			Role:    role,
			Content: string(content),
		})
	}

	// Sort rounds by number, then role (generative before adversarial).
	sort.Slice(result.Rounds, func(i, j int) bool {
		if result.Rounds[i].Number != result.Rounds[j].Number {
			return result.Rounds[i].Number < result.Rounds[j].Number
		}
		return result.Rounds[i].Role > result.Rounds[j].Role // "generative" > "adversarial"
	})

	return result, nil
}

// maxScoresFileSize caps the scores.jsonl file read to prevent OOM on
// unexpectedly large files (10 MiB is generous for JSONL score data).
const maxScoresFileSize = 10 << 20 // 10 MiB

// Scores reads scores/scores.jsonl, optionally filtering by pair name.
// Returns []parser.ScoreEntry sorted by timestamp descending.
// Returns an empty slice (not nil) when the file is missing or contains no matches.
// Malformed JSONL lines are skipped rather than causing a full failure.
func (f *FileDataSource) Scores(pair string) (any, error) {
	path := filepath.Join(f.rootDir, "scores", "scores.jsonl")

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []parser.ScoreEntry{}, nil
		}
		return nil, fmt.Errorf("stat scores.jsonl: %w", err)
	}
	// Guard: reject unexpectedly large files early to avoid allocating
	// unbounded memory when the JSONL file grows beyond operational norms.
	if info.Size() > maxScoresFileSize {
		return nil, fmt.Errorf("scores.jsonl too large: %d bytes (max %d)", info.Size(), maxScoresFileSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Race: file removed between stat and read.
			return []parser.ScoreEntry{}, nil
		}
		return nil, fmt.Errorf("read scores.jsonl: %w", err)
	}

	entries, skipped := parser.ParseScores(data)
	if skipped > 0 {
		slog.Warn("skipped malformed score lines in scores.jsonl", "skipped", skipped, "valid", len(entries))
	}

	if entries == nil {
		entries = []parser.ScoreEntry{}
	}

	if pair != "" {
		filtered := make([]parser.ScoreEntry, 0)
		for _, e := range entries {
			if e.Pair == pair {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Sort by timestamp descending (most recent first).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	return entries, nil
}

// PlanResponse wraps the parsed plan with the global regression budget from workflow.yaml.
type PlanResponse struct {
	*parser.Plan
	MaxRegressions int `json:"max_regressions"`
}

// Plan reads and parses plan.yaml, enriched with max_regressions from workflow.yaml.
// Returns a zero-value PlanResponse when plan.yaml is missing.
// Real I/O errors (e.g., permission denied) are still propagated.
func (f *FileDataSource) Plan() (any, error) {
	data, err := os.ReadFile(filepath.Join(f.rootDir, "plan.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Debug("plan.yaml not found, returning zero-value PlanResponse")
			return &PlanResponse{Plan: &parser.Plan{}, MaxRegressions: 2}, nil
		}
		return nil, fmt.Errorf("read plan.yaml: %w", err)
	}
	plan, err := parser.ParsePlan(data)
	if err != nil {
		return nil, fmt.Errorf("parse plan.yaml: %w", err)
	}

	maxRegressions := 2 // default when workflow.yaml absent or unparseable
	wfData, wfErr := os.ReadFile(filepath.Join(f.rootDir, "workflow.yaml"))
	if wfErr == nil {
		if wf, wfErr := parser.ParseWorkflow(wfData); wfErr == nil {
			maxRegressions = wf.MaxRegressions
		}
	}

	return &PlanResponse{Plan: plan, MaxRegressions: maxRegressions}, nil
}

// StatusInfo summarizes the current milestone, issue, and phase.
type StatusInfo struct {
	MilestoneID   int    `json:"milestone_id"`
	MilestoneName string `json:"milestone_name"`
	IssueRef      string `json:"issue_ref"`
	Phase         string `json:"phase"`
}

// Status derives the current milestone, issue, and phase from plan.yaml.
// Returns a zero-value StatusInfo when plan.yaml is missing.
// Real I/O errors (e.g., permission denied) are still propagated.
func (f *FileDataSource) Status() (any, error) {
	data, err := os.ReadFile(filepath.Join(f.rootDir, "plan.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Debug("plan.yaml not found, returning zero-value StatusInfo")
			return &StatusInfo{}, nil
		}
		return nil, fmt.Errorf("read plan.yaml: %w", err)
	}
	plan, err := parser.ParsePlan(data)
	if err != nil {
		return nil, fmt.Errorf("parse plan.yaml: %w", err)
	}

	info := &StatusInfo{}

	if plan.Epic.CurrentFocus != nil {
		info.MilestoneID = plan.Epic.CurrentFocus.MilestoneID
		info.IssueRef = plan.Epic.CurrentFocus.IssueRef
		info.Phase = plan.Epic.CurrentFocus.Phase
		// Find milestone name.
		for _, m := range plan.Epic.Milestones {
			if m.ID == plan.Epic.CurrentFocus.MilestoneID {
				info.MilestoneName = m.Name
				break
			}
		}
	}

	return info, nil
}
