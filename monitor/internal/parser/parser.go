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
	Version    int               `yaml:"version" json:"version"`
	MaxRounds  int               `yaml:"max_rounds" json:"max_rounds"`
	Escalation string            `yaml:"escalation" json:"escalation"`
	Progress   ProgressConfig    `yaml:"progress" json:"progress"`
	Components []ComponentConfig `yaml:"components" json:"components"`
	Pairs      []PairConfig      `yaml:"pairs" json:"pairs"`
	Guards     []GuardConfig     `yaml:"guards" json:"guards"`
}

// ProgressConfig describes the progress adapter settings.
type ProgressConfig struct {
	Adapter string `yaml:"adapter" json:"adapter"`
}

// ComponentConfig describes a workflow component.
type ComponentConfig struct {
	Name     string `yaml:"name" json:"name"`
	Scope    string `yaml:"scope" json:"scope"`
	Workflow string `yaml:"workflow" json:"workflow"`
}

// PairConfig describes a pair definition in workflow.yaml.
type PairConfig struct {
	Name      string `yaml:"name" json:"name"`
	Component string `yaml:"component" json:"component"`
	Phase     string `yaml:"phase" json:"phase"`
	Scope     string `yaml:"scope" json:"scope"`
	Enabled   bool   `yaml:"enabled" json:"enabled"`
}

// GuardConfig describes a guard in workflow.yaml.
type GuardConfig struct {
	Name        string `yaml:"name" json:"name"`
	Command     string `yaml:"command" json:"command"`
	Expect      string `yaml:"expect" json:"expect"`
	Phase       string `yaml:"phase" json:"phase"`
	Description string `yaml:"description" json:"description"`
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
}

// Milestone represents one milestone in the plan.
type Milestone struct {
	ID          int               `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Pairs       []string          `yaml:"pairs" json:"pairs"`
	Status      string            `yaml:"status" json:"status"`
	PhaseStatus map[string]string `yaml:"phase_status" json:"phase_status"`
	DoneWhen    string            `yaml:"done_when" json:"done_when"`
	ProgressRef *string           `yaml:"progress_ref" json:"progress_ref"`
}

// CurrentFocus describes the current working focus.
type CurrentFocus struct {
	MilestoneID int    `yaml:"milestone_id" json:"milestone_id"`
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

// ParseWorkflow parses a workflow.yaml file from raw bytes.
func ParseWorkflow(data []byte) (*WorkflowConfig, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return &WorkflowConfig{}, nil
	}
	var wf WorkflowConfig
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow: %w", err)
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
