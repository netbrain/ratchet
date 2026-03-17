package client

import "time"

// PairStatus represents a ratchet pair and its current state.
type PairStatus struct {
	Name      string `json:"name"`
	Component string `json:"component"`
	Phase     string `json:"phase"`
	Scope     string `json:"scope"`
	Enabled   bool   `json:"enabled"`
	Active    bool   `json:"active"`
	Status    string `json:"status"`
}

// DebateMeta contains metadata about a debate.
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

// Round represents a single round in a debate.
type Round struct {
	Number  int    `json:"number"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DebateWithRounds is a debate with its full round history.
type DebateWithRounds struct {
	DebateMeta
	Rounds []Round `json:"rounds"`
}

// Plan represents the ratchet epic plan.
type Plan struct {
	Epic EpicConfig `json:"epic"`
}

// EpicConfig holds the epic configuration.
type EpicConfig struct {
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Milestones   []Milestone   `json:"milestones"`
	CurrentFocus *CurrentFocus `json:"current_focus"`
	Discoveries  []Discovery   `json:"discoveries"`
}

// Discovery represents a sidequest or discovery found during execution.
type Discovery struct {
	Ref         string `json:"ref"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Source      string `json:"source"`
	CreatedAt   string `json:"created_at"`
}

// Milestone represents a single milestone in the plan.
type Milestone struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Pairs       []string          `json:"pairs"`
	Status      string            `json:"status"`
	PhaseStatus map[string]string `json:"phase_status"`
	DoneWhen    string            `json:"done_when"`
	ProgressRef *string           `json:"progress_ref"`
	DependsOn   []int             `json:"depends_on"`
	Regressions    int               `json:"regressions"`
	MaxRegressions int               `json:"max_regressions"`
	Issues         []Issue           `json:"issues"`
}

// Issue represents a single issue within a milestone (v2 only).
type Issue struct {
	Ref         string            `json:"ref"`
	Title       string            `json:"title"`
	Pairs       []string          `json:"pairs"`
	DependsOn   []string          `json:"depends_on"`
	PhaseStatus map[string]string `json:"phase_status"`
	Files       []string          `json:"files"`
	Debates     []string          `json:"debates"`
	Branch      *string           `json:"branch"`
	Status      string            `json:"status"`
}

// CurrentFocus indicates what the system is currently working on.
type CurrentFocus struct {
	MilestoneID int    `json:"milestone_id"`
	Phase       string `json:"phase"`
	Started     string `json:"started"`
}

// StatusInfo represents the current system status.
type StatusInfo struct {
	MilestoneID   int    `json:"milestone_id"`
	MilestoneName string `json:"milestone_name"`
	IssueRef      string `json:"issue_ref"`
	Phase         string `json:"phase"`
}

// ScoreEntry represents a single score record.
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

// Workspace represents a configured workspace.
type Workspace struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// HealthStatus represents the health check response.
type HealthStatus struct {
	Status string `json:"status"`
}

// SSEEvent represents a parsed server-sent event.
type SSEEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data []byte `json:"data"`
}

// ConnectionState represents the client's connection state.
type ConnectionState int

const (
	Disconnected ConnectionState = iota
	Connected
	Reconnecting
)

func (s ConnectionState) String() string {
	switch s {
	case Disconnected:
		return "disconnected"
	case Connected:
		return "connected"
	case Reconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}
