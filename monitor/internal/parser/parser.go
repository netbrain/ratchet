// Package parser provides types and parsing functions for all .ratchet/ file formats.
package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrEmptyInput is returned when a parser receives zero-length data for a
// format that requires content (e.g., JSON).
var ErrEmptyInput = errors.New("empty input")

// WorkflowConfig represents the parsed workflow.yaml file.
type WorkflowConfig struct {
	Version        int               `yaml:"version" json:"version"`
	MaxRounds      int               `yaml:"max_rounds" json:"max_rounds"`
	Escalation     string            `yaml:"escalation" json:"escalation"`
	PRScope        string            `yaml:"pr_scope" json:"pr_scope"`
	MaxRegressions int               `yaml:"max_regressions" json:"max_regressions"`
	Progress       ProgressConfig    `yaml:"progress" json:"progress"`
	Workspaces     []WorkspaceConfig `yaml:"workspaces" json:"workspaces"`
	Models         ModelsConfig      `yaml:"models" json:"models"`
	Components     []ComponentConfig `yaml:"components" json:"components"`
	Pairs          []PairConfig      `yaml:"pairs" json:"pairs"`
	Guards         []GuardConfig     `yaml:"guards" json:"guards"`
	Resources      []ResourceConfig  `yaml:"resources" json:"resources"`
}

// ProgressConfig describes the progress adapter settings.
type ProgressConfig struct {
	Adapter string `yaml:"adapter" json:"adapter"`
}

// WorkspaceConfig describes a workspace in workflow.yaml.
type WorkspaceConfig struct {
	Path string `yaml:"path" json:"path"`
	Name string `yaml:"name" json:"name"`
}

// ModelsConfig describes model assignments in workflow.yaml.
type ModelsConfig struct {
	DebateRunner string `yaml:"debate_runner" json:"debate_runner"`
	Generative   string `yaml:"generative" json:"generative"`
	Adversarial  string `yaml:"adversarial" json:"adversarial"`
	Tiebreaker   string `yaml:"tiebreaker" json:"tiebreaker"`
	Analyst      string `yaml:"analyst" json:"analyst"`
}

// ResourceConfig describes a shared resource in workflow.yaml.
type ResourceConfig struct {
	Name      string `yaml:"name" json:"name"`
	Start     string `yaml:"start" json:"start"`
	Stop      string `yaml:"stop" json:"stop"`
	Singleton bool   `yaml:"singleton" json:"singleton"`
}

// ComponentConfig describes a workflow component.
type ComponentConfig struct {
	Name     string `yaml:"name" json:"name"`
	Scope    string `yaml:"scope" json:"scope"`
	Workflow string `yaml:"workflow" json:"workflow"`
}

// PairConfig describes a pair definition in workflow.yaml.
type PairConfig struct {
	Name      string           `yaml:"name" json:"name"`
	Component string           `yaml:"component" json:"component"`
	Phase     string           `yaml:"phase" json:"phase"`
	Scope     string           `yaml:"scope" json:"scope"`
	Enabled   bool             `yaml:"enabled" json:"enabled"`
	MaxRounds int              `yaml:"max_rounds" json:"max_rounds"`
	Models    PairModelsConfig `yaml:"models" json:"models"`
}

// PairModelsConfig describes per-pair model overrides.
type PairModelsConfig struct {
	Generative  string `yaml:"generative" json:"generative"`
	Adversarial string `yaml:"adversarial" json:"adversarial"`
}

// GuardConfig describes a guard in workflow.yaml.
type GuardConfig struct {
	Name        string   `yaml:"name" json:"name"`
	Command     string   `yaml:"command" json:"command"`
	Expect      string   `yaml:"expect" json:"expect"`
	Phase       string   `yaml:"phase" json:"phase"`
	Description string   `yaml:"description" json:"description"`
	Blocking    bool     `yaml:"blocking" json:"blocking"`
	Timing      string   `yaml:"timing" json:"timing"`
	Components  []string `yaml:"components" json:"components"`
	Requires    []string `yaml:"requires" json:"requires"`
}

// Plan represents the parsed plan.yaml file.
type Plan struct {
	Epic EpicConfig `yaml:"epic" json:"epic"`
}

// EpicConfig is the top-level epic in plan.yaml.
type EpicConfig struct {
	Name         string        `yaml:"name" json:"name"`
	Description  string        `yaml:"description" json:"description"`
	Milestones   []Milestone   `yaml:"milestones" json:"milestones"`
	CurrentFocus *CurrentFocus `yaml:"current_focus" json:"current_focus"`
	Discoveries  []Discovery   `yaml:"discoveries" json:"discoveries"`
}

// Discovery represents a sidequest or discovery found during execution.
type Discovery struct {
	Ref         string `yaml:"ref" json:"ref"`
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`
	Source      string `yaml:"source" json:"source"`
	CreatedAt   string `yaml:"created_at" json:"created_at"`
}

// Milestone represents one milestone in the plan.
type Milestone struct {
	ID          int               `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Pairs       []string          `yaml:"pairs" json:"pairs"` // v1 field, deprecated in v2
	Status      string            `yaml:"status" json:"status"`
	PhaseStatus map[string]string `yaml:"phase_status" json:"phase_status"` // v1 field, deprecated in v2
	DoneWhen    string            `yaml:"done_when" json:"done_when"`
	ProgressRef *string           `yaml:"progress_ref" json:"progress_ref"`
	// v2 fields
	DependsOn   []int   `yaml:"depends_on" json:"depends_on"`   // milestone IDs this depends on
	Regressions int     `yaml:"regressions" json:"regressions"` // regression budget counter
	Issues      []Issue `yaml:"issues" json:"issues"`           // issues within this milestone
}

// Issue represents a single issue within a milestone (v2 only).
type Issue struct {
	Ref         string            `yaml:"ref" json:"ref"`                   // unique reference like "issue-1-1"
	Title       string            `yaml:"title" json:"title"`               // human-readable title
	Pairs       []string          `yaml:"pairs" json:"pairs"`               // pair names for this issue
	DependsOn   []string          `yaml:"depends_on" json:"depends_on"`     // issue refs this depends on
	PhaseStatus map[string]string `yaml:"phase_status" json:"phase_status"` // status per phase
	Files       []string          `yaml:"files" json:"files"`               // modified files
	Debates     []string          `yaml:"debates" json:"debates"`           // debate IDs
	Branch      *string           `yaml:"branch" json:"branch"`             // git branch name
	Status      string            `yaml:"status" json:"status"`             // overall status
}

// CurrentFocus describes the current working focus.
type CurrentFocus struct {
	MilestoneID int    `yaml:"milestone_id" json:"milestone_id"`
	IssueRef    string `yaml:"issue_ref" json:"issue_ref"`
	Phase       string `yaml:"phase" json:"phase"`
	Started     string `yaml:"started" json:"started"`
}

// ProjectConfig represents the parsed project.yaml file.
type ProjectConfig struct {
	Name         string             `yaml:"name" json:"name"`
	Description  string             `yaml:"description" json:"description"`
	Stack        StackConfig        `yaml:"stack" json:"stack"`
	Architecture ArchitectureConfig `yaml:"architecture" json:"architecture"`
	Testing      TestingConfig      `yaml:"testing" json:"testing"`
}

// StackConfig describes the technology stack.
type StackConfig struct {
	Language  string `yaml:"language" json:"language"`
	Version   string `yaml:"version" json:"version"`
	Frontend  string `yaml:"frontend" json:"frontend"`
	Transport string `yaml:"transport" json:"transport"`
	Templates string `yaml:"templates" json:"templates"`
	Build     string `yaml:"build" json:"build"`
}

// ArchitectureConfig describes the architecture.
type ArchitectureConfig struct {
	Pattern    string          `yaml:"pattern" json:"pattern"`
	Components []ArchComponent `yaml:"components" json:"components"`
	DataFlow   string          `yaml:"data_flow" json:"data_flow"`
}

// ArchComponent is a component within the architecture config.
type ArchComponent struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Scope       string `yaml:"scope" json:"scope"`
}

// TestingConfig describes the testing configuration.
type TestingConfig struct {
	Framework string            `yaml:"framework" json:"framework"`
	Strategy  string            `yaml:"strategy" json:"strategy"`
	Commands  map[string]string `yaml:"commands" json:"commands"`
}

// DebateMeta represents a parsed debate meta.json file.
type DebateMeta struct {
	ID         string     `json:"id"`
	Pair       string     `json:"pair"`
	Phase      string     `json:"phase"`
	Milestone  int        `json:"milestone"`
	Files      []string   `json:"files"`
	Status     string     `json:"status"`
	RoundCount int        `json:"round_count"`
	MaxRounds  int        `json:"max_rounds"`
	Started    time.Time  `json:"started"`
	Resolved   *time.Time `json:"resolved"`
	Verdict    *string    `json:"verdict"`
}

// ScoreEntry represents a single line from scores.jsonl.
type ScoreEntry struct {
	Timestamp         time.Time `json:"timestamp"`
	DebateID          string    `json:"debate_id"`
	Pair              string    `json:"pair"`
	Milestone         int       `json:"milestone"`
	RoundsToConsensus int       `json:"rounds_to_consensus"`
	Escalated         bool      `json:"escalated"`
	IssuesFound       int       `json:"issues_found"`
	IssuesResolved    int       `json:"issues_resolved"`
}

// PairDefinition represents a parsed pair markdown file.
type PairDefinition struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// validPRScopes is the set of allowed values for pr_scope.
var validPRScopes = map[string]bool{
	"debate":    true,
	"phase":     true,
	"milestone": true,
	"issue":     true,
}

// ParseWorkflow parses a workflow.yaml file from raw bytes.
func ParseWorkflow(data []byte) (*WorkflowConfig, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return &WorkflowConfig{}, nil
	}

	// Pre-parse into a generic map for validation of required fields and types.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse workflow: %w", err)
	}

	// If the document parsed but has content, validate required fields and types.
	if raw != nil {
		// Check required fields: version, max_rounds, escalation.
		// We only enforce required fields when at least one of them is present
		// (i.e., the document looks like a workflow config, not an empty/whitespace doc).
		hasVersion := raw["version"] != nil
		hasMaxRounds := raw["max_rounds"] != nil
		hasEscalation := raw["escalation"] != nil

		if hasVersion || hasMaxRounds || hasEscalation {
			if !hasVersion {
				return nil, fmt.Errorf("parse workflow: missing required field: version")
			}
			if !hasMaxRounds {
				return nil, fmt.Errorf("parse workflow: missing required field: max_rounds")
			}
			if !hasEscalation {
				return nil, fmt.Errorf("parse workflow: missing required field: escalation")
			}
		}

		// Type validation: escalation must be a string (YAML may coerce int to string).
		if v, ok := raw["escalation"]; ok && v != nil {
			switch v.(type) {
			case string:
				// OK
			default:
				return nil, fmt.Errorf("parse workflow: escalation must be a string, got %T", v)
			}
		}

		// Type validation: max_rounds must be an int.
		if v, ok := raw["max_rounds"]; ok && v != nil {
			switch v.(type) {
			case int:
				// OK
			default:
				return nil, fmt.Errorf("parse workflow: max_rounds must be an integer, got %T", v)
			}
		}

		// Type validation: max_regressions must be an int if present.
		if v, ok := raw["max_regressions"]; ok && v != nil {
			switch v.(type) {
			case int:
				// OK
			default:
				return nil, fmt.Errorf("parse workflow: max_regressions must be an integer, got %T", v)
			}
		}
	}

	var wf WorkflowConfig
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow: %w", err)
	}

	// Apply defaults
	if wf.PRScope == "" {
		wf.PRScope = "issue"
	}

	// Validate pr_scope enum
	if !validPRScopes[wf.PRScope] {
		return nil, fmt.Errorf("parse workflow: invalid pr_scope value %q; must be one of: debate, phase, milestone, issue", wf.PRScope)
	}

	// Apply max_regressions default only when not explicitly set.
	// Check the raw map to distinguish "absent" from "explicitly zero".
	if raw != nil {
		if _, explicit := raw["max_regressions"]; !explicit {
			wf.MaxRegressions = 2
		}
	} else {
		wf.MaxRegressions = 2
	}

	// Apply guard timing defaults
	for i := range wf.Guards {
		if wf.Guards[i].Timing == "" {
			wf.Guards[i].Timing = "post-debate"
		}
	}

	return &wf, nil
}

// ParsePlan parses a plan.yaml file from raw bytes.
func ParsePlan(data []byte) (*Plan, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return &Plan{}, nil
	}
	var p Plan
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}
	return &p, nil
}

// ParseProject parses a project.yaml file from raw bytes.
func ParseProject(data []byte) (*ProjectConfig, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return &ProjectConfig{}, nil
	}
	var pc ProjectConfig
	if err := yaml.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("parse project: %w", err)
	}
	return &pc, nil
}

// ParseDebateMeta parses a debate meta.json file from raw bytes.
func ParseDebateMeta(data []byte) (*DebateMeta, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("parse debate meta: %w", ErrEmptyInput)
	}
	var dm DebateMeta
	if err := json.Unmarshal(data, &dm); err != nil {
		return nil, fmt.Errorf("parse debate meta: %w", err)
	}
	return &dm, nil
}

// ParseScoreEntry parses a single scores.jsonl line from raw bytes.
func ParseScoreEntry(data []byte) (*ScoreEntry, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("parse score entry: %w", ErrEmptyInput)
	}
	var se ScoreEntry
	if err := json.Unmarshal(data, &se); err != nil {
		return nil, fmt.Errorf("parse score entry: %w", err)
	}
	return &se, nil
}

// ParseScores parses all lines from a scores.jsonl file.
// Malformed lines are skipped (logged at debug level by the caller)
// rather than failing the entire parse, matching the resilience model
// used by Debates() for malformed meta.json files.
func ParseScores(data []byte) ([]ScoreEntry, int) {
	if len(data) == 0 {
		return []ScoreEntry{}, 0
	}
	var entries []ScoreEntry
	var skipped int
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		entry, err := ParseScoreEntry(line)
		if err != nil {
			skipped++
			continue
		}
		entries = append(entries, *entry)
	}
	return entries, skipped
}

// Round represents a single debate round parsed from a round-{N}-{role}.md file.
type Round struct {
	Number  int    `json:"number"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DebateWithRounds extends DebateMeta with the full round content.
type DebateWithRounds struct {
	DebateMeta
	Rounds []Round `json:"rounds"`
}

// ParseRoundFilename extracts the round number and role from a filename
// matching the pattern round-{N}-{role}.md. It returns an error if the
// filename does not match the expected format.
func ParseRoundFilename(name string) (int, string, error) {
	// Strip .md extension.
	if !strings.HasSuffix(name, ".md") {
		return 0, "", fmt.Errorf("parse round filename: expected .md extension, got %q", name)
	}
	base := strings.TrimSuffix(name, ".md")

	// Expect "round-{N}-{role}"
	if !strings.HasPrefix(base, "round-") {
		return 0, "", fmt.Errorf("parse round filename: expected round- prefix, got %q", name)
	}
	rest := strings.TrimPrefix(base, "round-")

	// Split into number and role at the first dash.
	idx := strings.Index(rest, "-")
	if idx < 0 {
		return 0, "", fmt.Errorf("parse round filename: missing role in %q", name)
	}

	numStr := rest[:idx]
	role := rest[idx+1:]

	if role == "" {
		return 0, "", fmt.Errorf("parse round filename: empty role in %q", name)
	}

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, "", fmt.Errorf("parse round filename: invalid number in %q: %w", name, err)
	}
	if num < 0 {
		return 0, "", fmt.Errorf("parse round filename: negative round number in %q", name)
	}

	return num, role, nil
}

// ParsePairDefinition parses a pair markdown file, extracting the name from
// the first H1 heading and the full content.
func ParsePairDefinition(data []byte) (*PairDefinition, error) {
	if len(data) == 0 {
		return &PairDefinition{}, nil
	}
	content := string(data)
	pd := &PairDefinition{
		Content: content,
	}

	// Extract name from first H1 heading (line starting with "# ").
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			pd.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			break
		}
	}

	return pd, nil
}
