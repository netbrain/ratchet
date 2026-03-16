// Package state manages the TUI application state.
package state

import (
	"strings"
	"sync"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
)

// Store holds the full UI state and is safe for concurrent access.
type Store struct {
	mu sync.RWMutex

	pairs        []client.PairStatus
	debates      []client.DebateMeta
	debateDetail *client.DebateWithRounds
	plan         client.Plan
	status       client.StatusInfo
	scores       map[string][]client.ScoreEntry
	connState    client.ConnectionState
	lastEventID  string
	dirty        map[string]bool // resource -> needs refresh
}

// NewStore returns an initialised Store.
func NewStore() *Store {
	return &Store{
		scores: make(map[string][]client.ScoreEntry),
		dirty:  make(map[string]bool),
	}
}

// ── Pairs ─────────────────────────────────────────────────────────────

func (s *Store) SetPairs(p []client.PairStatus) {
	s.mu.Lock()
	s.pairs = p
	s.mu.Unlock()
}

func (s *Store) Pairs() []client.PairStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]client.PairStatus, len(s.pairs))
	copy(out, s.pairs)
	return out
}

// ── Debates ───────────────────────────────────────────────────────────

func (s *Store) SetDebates(d []client.DebateMeta) {
	s.mu.Lock()
	s.debates = d
	s.mu.Unlock()
}

func (s *Store) Debates() []client.DebateMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]client.DebateMeta, len(s.debates))
	copy(out, s.debates)
	return out
}

// ── Debate Detail ───────────────────────────────────────────────────

func (s *Store) SetDebateDetail(d *client.DebateWithRounds) {
	s.mu.Lock()
	s.debateDetail = d
	s.mu.Unlock()
}

func (s *Store) DebateDetail(id string) *client.DebateWithRounds {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.debateDetail != nil && s.debateDetail.ID == id {
		return s.debateDetail
	}
	return nil
}

// ── Plan ──────────────────────────────────────────────────────────────

func (s *Store) SetPlan(p client.Plan) {
	s.mu.Lock()
	s.plan = p
	s.mu.Unlock()
}

func (s *Store) Plan() client.Plan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.plan
}

// ── Status ────────────────────────────────────────────────────────────

func (s *Store) SetStatus(st client.StatusInfo) {
	s.mu.Lock()
	s.status = st
	s.mu.Unlock()
}

func (s *Store) Status() client.StatusInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// ── Scores ────────────────────────────────────────────────────────────

func (s *Store) SetScores(pair string, sc []client.ScoreEntry) {
	s.mu.Lock()
	s.scores[pair] = sc
	s.mu.Unlock()
}

func (s *Store) Scores(pair string) []client.ScoreEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.scores[pair]
	if !ok {
		return []client.ScoreEntry{}
	}
	out := make([]client.ScoreEntry, len(sc))
	copy(out, sc)
	return out
}

// ── Connection state ──────────────────────────────────────────────────

func (s *Store) SetConnectionState(cs client.ConnectionState) {
	s.mu.Lock()
	s.connState = cs
	s.mu.Unlock()
}

func (s *Store) ConnectionState() client.ConnectionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connState
}

// ── SSE event handling ────────────────────────────────────────────────

// ApplyEvent processes an SSE event and marks the appropriate resource as dirty.
func (s *Store) ApplyEvent(ev client.SSEEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ev.ID != "" {
		s.lastEventID = ev.ID
	}

	resource := resourceForEvent(ev.Type)
	if resource != "" {
		s.dirty[resource] = true
	}

	return nil
}

// NeedsRefresh reports whether the given resource has been marked dirty by
// an SSE event and resets the flag.
func (s *Store) NeedsRefresh(resource string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dirty[resource] {
		s.dirty[resource] = false
		return true
	}
	return false
}

// LastEventID returns the ID of the most recently applied SSE event.
func (s *Store) LastEventID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastEventID
}

// resourceForEvent maps an SSE event type to the resource that should be refreshed.
func resourceForEvent(eventType string) string {
	prefix, _, _ := strings.Cut(eventType, ":")
	switch prefix {
	case "debate":
		return "debates"
	case "score":
		return "scores"
	case "pair":
		return "pairs"
	case "plan":
		return "plan"
	case "config":
		return "config"
	default:
		return ""
	}
}
