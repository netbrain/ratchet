package views

import (
	"fmt"
	"maps"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// IssueStatus holds display data for a single issue within a milestone.
type IssueStatus struct {
	Ref         string
	Title       string
	Pairs       []string
	DependsOn   []string
	PhaseStatus map[string]string
	Status      string
	Files       []string
	Debates     []string
}

// MilestoneStatus holds display data for a single milestone.
type MilestoneStatus struct {
	ID             int
	Name           string
	Status         string
	PhaseStatus    map[string]string
	DoneWhen       string
	DependsOn      []int
	Regressions    int
	MaxRegressions int // budget threshold for warning colors
	Layer          int // DAG layer (0 = no dependencies, 1+ = dependent on earlier layers)
	Issues         []IssueStatus
}

// EpicViewModel is the view model for the epic status tab.
type EpicViewModel struct {
	store          *state.Store
	plan           client.Plan
	milestones     []MilestoneStatus
	selected       int
	viewportHeight int
	scrollOffset   int
}

// NewEpicViewModel creates an EpicViewModel backed by the given store.
func NewEpicViewModel(store *state.Store) *EpicViewModel {
	vm := &EpicViewModel{store: store}
	vm.loadPlan()
	return vm
}

// EpicName returns the epic's name.
func (vm *EpicViewModel) EpicName() string {
	if vm == nil {
		return ""
	}
	return vm.plan.Epic.Name
}

// EpicDescription returns the epic's description.
func (vm *EpicViewModel) EpicDescription() string {
	if vm == nil {
		return ""
	}
	return vm.plan.Epic.Description
}

// Milestones returns all milestones with their status.
func (vm *EpicViewModel) Milestones() []MilestoneStatus {
	if vm == nil {
		return nil
	}
	return vm.milestones
}

// CompletedCount returns the number of done milestones.
func (vm *EpicViewModel) CompletedCount() int {
	if vm == nil {
		return 0
	}
	count := 0
	for _, m := range vm.milestones {
		if m.Status == "done" {
			count++
		}
	}
	return count
}

// TotalCount returns the total number of milestones.
func (vm *EpicViewModel) TotalCount() int {
	if vm == nil {
		return 0
	}
	return len(vm.milestones)
}

// ProgressPercent returns the completion fraction (0.0-1.0).
func (vm *EpicViewModel) ProgressPercent() float64 {
	if vm == nil || len(vm.milestones) == 0 {
		return 0.0
	}
	return float64(vm.CompletedCount()) / float64(len(vm.milestones))
}

// CurrentFocus returns the current focus, or nil if not set.
func (vm *EpicViewModel) CurrentFocus() *client.CurrentFocus {
	if vm == nil {
		return nil
	}
	return vm.plan.Epic.CurrentFocus
}

// RegressionBudgetText returns the formatted regression budget string "X/Y".
func (vm *EpicViewModel) RegressionBudgetText(m MilestoneStatus) string {
	maxReg := m.MaxRegressions
	if maxReg <= 0 {
		maxReg = 2 // default budget
	}
	return fmt.Sprintf("%d/%d", m.Regressions, maxReg)
}

// RegressionWarningLevel returns the warning level for a milestone's regressions.
// "none" = within budget, "warn" = at or near budget, "danger" = over budget.
func (vm *EpicViewModel) RegressionWarningLevel(m MilestoneStatus) string {
	if vm == nil {
		return "none"
	}
	maxReg := m.MaxRegressions
	if maxReg <= 0 {
		maxReg = 2 // default budget
	}
	if m.Regressions >= maxReg {
		return "danger"
	}
	if m.Regressions > 0 && m.Regressions >= maxReg-1 {
		return "warn"
	}
	return "none"
}

// MilestoneStatusColor maps a milestone status to a color name.
func (vm *EpicViewModel) MilestoneStatusColor(status string) string {
	switch status {
	case "pending":
		return "dim"
	case "in_progress":
		return "cyan"
	case "done":
		return "green"
	default:
		return "white"
	}
}

// SelectedIndex returns the current selection index.
func (vm *EpicViewModel) SelectedIndex() int {
	if vm == nil {
		return 0
	}
	return vm.selected
}

// SelectNext moves selection forward with wrap-around.
func (vm *EpicViewModel) SelectNext() {
	if vm == nil {
		return
	}
	n := len(vm.milestones)
	if n == 0 {
		return
	}
	vm.selected = (vm.selected + 1) % n
	vm.adjustScrollOffset()
}

// SelectPrev moves selection backward with wrap-around.
func (vm *EpicViewModel) SelectPrev() {
	if vm == nil {
		return
	}
	n := len(vm.milestones)
	if n == 0 {
		return
	}
	vm.selected = (vm.selected - 1 + n) % n
	vm.adjustScrollOffset()
}

// SelectFirst jumps to the first milestone.
func (vm *EpicViewModel) SelectFirst() {
	if vm == nil {
		return
	}
	vm.selected = 0
	vm.adjustScrollOffset()
}

// SelectLast jumps to the last milestone.
func (vm *EpicViewModel) SelectLast() {
	if vm == nil {
		return
	}
	n := len(vm.milestones)
	if n == 0 {
		return
	}
	vm.selected = n - 1
	vm.adjustScrollOffset()
}

// SelectedMilestone returns the currently selected milestone, or nil if empty.
func (vm *EpicViewModel) SelectedMilestone() *MilestoneStatus {
	if vm == nil || len(vm.milestones) == 0 {
		return nil
	}
	ms := vm.milestones[vm.selected]
	return &ms
}

// SetViewportHeight sets the viewport height and recalculates scroll offset.
func (vm *EpicViewModel) SetViewportHeight(h int) {
	if vm == nil {
		return
	}
	vm.viewportHeight = h
	vm.adjustScrollOffset()
}

// ScrollOffset returns the current scroll offset.
func (vm *EpicViewModel) ScrollOffset() int {
	if vm == nil {
		return 0
	}
	return vm.scrollOffset
}

// Refresh re-reads the plan from the store.
func (vm *EpicViewModel) Refresh() {
	if vm == nil {
		return
	}
	vm.loadPlan()
	vm.clampSelection()
	vm.adjustScrollOffset()
}

// MilestonesByLayer groups milestones by their DAG layer for rendering.
// Returns a map of layer number to milestones in that layer.
func (vm *EpicViewModel) MilestonesByLayer() map[int][]MilestoneStatus {
	if vm == nil {
		return nil
	}
	byLayer := make(map[int][]MilestoneStatus)
	for _, m := range vm.milestones {
		byLayer[m.Layer] = append(byLayer[m.Layer], m)
	}
	return byLayer
}

// MaxLayer returns the highest DAG layer number, or -1 if no milestones.
func (vm *EpicViewModel) MaxLayer() int {
	if vm == nil || len(vm.milestones) == 0 {
		return -1
	}
	maxLayer := 0
	for _, m := range vm.milestones {
		if m.Layer > maxLayer {
			maxLayer = m.Layer
		}
	}
	return maxLayer
}

// DAGConnector describes a visual connector between milestone layers.
type DAGConnector struct {
	FromID int
	ToID   int
	Symbol string // e.g., "│", "├", "└"
}

// DAGConnectors returns connectors for rendering between layers.
func (vm *EpicViewModel) DAGConnectors() []DAGConnector {
	if vm == nil || len(vm.milestones) == 0 {
		return nil
	}
	var connectors []DAGConnector
	for _, m := range vm.milestones {
		for _, depID := range m.DependsOn {
			symbol := "│"
			connectors = append(connectors, DAGConnector{
				FromID: depID,
				ToID:   m.ID,
				Symbol: symbol,
			})
		}
	}
	return connectors
}

// DAGPrefix returns a visual prefix string for a milestone indicating its DAG relationships.
// Uses box-drawing characters to show dependency structure:
//   - "┌─" for the first milestone in a layer group
//   - "├─" for middle milestones in a layer group
//   - "└─" for the last milestone in the last layer, or the last in a group with more layers below
//   - "│ " for continuation when there are more items below (not applicable here — used in connectors)
func (vm *EpicViewModel) DAGPrefix(m MilestoneStatus) string {
	if vm == nil {
		return ""
	}
	maxLayer := vm.MaxLayer()
	if maxLayer <= 0 {
		// No DAG structure (all layer 0 or single layer) — no connectors needed
		return "  "
	}

	// Count milestones in this layer and find position
	posInLayer := 0
	countInLayer := 0
	for _, ms := range vm.milestones {
		if ms.Layer == m.Layer {
			if ms.ID == m.ID {
				posInLayer = countInLayer
			}
			countInLayer++
		}
	}

	isLastLayer := m.Layer == maxLayer
	isFirstInLayer := posInLayer == 0
	isLastInLayer := posInLayer == countInLayer-1

	if isLastLayer && isLastInLayer {
		return "└─"
	}
	if isFirstInLayer && m.Layer == 0 {
		return "┌─"
	}
	if isLastInLayer && !isLastLayer {
		return "├─"
	}
	return "├─"
}

// DAGLayerLabel returns the layer label string for a given layer number (e.g., "L0", "L1").
func (vm *EpicViewModel) DAGLayerLabel(layer int) string {
	if vm == nil {
		return ""
	}
	return fmt.Sprintf("L%d", layer)
}

// DAGLayout returns milestones sorted by layer for rendering, along with
// metadata about where layer boundaries occur. This is the primary method
// for rendering the DAG visualization.
// Returns nil if there are no milestones.
func (vm *EpicViewModel) DAGLayout() []DAGLayoutEntry {
	if vm == nil || len(vm.milestones) == 0 {
		return nil
	}

	maxLayer := vm.MaxLayer()
	var layout []DAGLayoutEntry

	for layer := 0; layer <= maxLayer; layer++ {
		var layerMilestones []MilestoneStatus
		for _, m := range vm.milestones {
			if m.Layer == layer {
				layerMilestones = append(layerMilestones, m)
			}
		}
		for i, m := range layerMilestones {
			layout = append(layout, DAGLayoutEntry{
				Milestone:    m,
				Layer:        layer,
				IsFirstInLayer: i == 0,
				IsLastInLayer:  i == len(layerMilestones)-1,
				IsLastLayer:    layer == maxLayer,
			})
		}
	}

	return layout
}

// DAGLayoutEntry describes a single milestone's position in the DAG layout.
type DAGLayoutEntry struct {
	Milestone      MilestoneStatus
	Layer          int
	IsFirstInLayer bool
	IsLastInLayer  bool
	IsLastLayer    bool
}

// HasDAG returns true if the milestone graph has any dependency structure (more than one layer).
func (vm *EpicViewModel) HasDAG() bool {
	if vm == nil {
		return false
	}
	return vm.MaxLayer() > 0
}

// IsBlocked returns true if the milestone has unmet dependencies (any dep not "done").
func (vm *EpicViewModel) IsBlocked(m MilestoneStatus) bool {
	if vm == nil || len(m.DependsOn) == 0 {
		return false
	}
	if m.Status == "done" {
		return false
	}
	statusByID := make(map[int]string, len(vm.milestones))
	for _, ms := range vm.milestones {
		statusByID[ms.ID] = ms.Status
	}
	for _, depID := range m.DependsOn {
		if statusByID[depID] != "done" {
			return true
		}
	}
	return false
}

func (vm *EpicViewModel) clampSelection() {
	n := len(vm.milestones)
	if n == 0 {
		vm.selected = 0
	} else if vm.selected >= n {
		vm.selected = n - 1
	}
}

func (vm *EpicViewModel) adjustScrollOffset() {
	n := len(vm.milestones)
	if vm.viewportHeight <= 0 || n <= vm.viewportHeight {
		vm.scrollOffset = 0
		return
	}
	if vm.selected < vm.scrollOffset {
		vm.scrollOffset = vm.selected
	}
	if vm.selected >= vm.scrollOffset+vm.viewportHeight {
		vm.scrollOffset = vm.selected - vm.viewportHeight + 1
	}
}

func (vm *EpicViewModel) loadPlan() {
	if vm.store == nil {
		vm.plan = client.Plan{}
		vm.milestones = nil
		return
	}
	vm.plan = vm.store.Plan()
	vm.milestones = nil

	// Build milestones with DAG layer calculation
	layers := vm.calculateDAGLayers(vm.plan.Epic.Milestones)

	for _, m := range vm.plan.Epic.Milestones {
		// Deep copy PhaseStatus to avoid sharing the map with the store.
		ps := make(map[string]string, len(m.PhaseStatus))
		maps.Copy(ps, m.PhaseStatus)

		// Deep copy DependsOn
		deps := make([]int, len(m.DependsOn))
		copy(deps, m.DependsOn)

		// Deep copy Issues
		var issues []IssueStatus
		for _, iss := range m.Issues {
			ips := make(map[string]string, len(iss.PhaseStatus))
			maps.Copy(ips, iss.PhaseStatus)

			pairs := make([]string, len(iss.Pairs))
			copy(pairs, iss.Pairs)

			depOn := make([]string, len(iss.DependsOn))
			copy(depOn, iss.DependsOn)

			files := make([]string, len(iss.Files))
			copy(files, iss.Files)

			debates := make([]string, len(iss.Debates))
			copy(debates, iss.Debates)

			issues = append(issues, IssueStatus{
				Ref:         iss.Ref,
				Title:       iss.Title,
				Pairs:       pairs,
				DependsOn:   depOn,
				PhaseStatus: ips,
				Status:      iss.Status,
				Files:       files,
				Debates:     debates,
			})
		}

		vm.milestones = append(vm.milestones, MilestoneStatus{
			ID:             m.ID,
			Name:           m.Name,
			Status:         m.Status,
			PhaseStatus:    ps,
			DoneWhen:       m.DoneWhen,
			DependsOn:      deps,
			Regressions:    m.Regressions,
			MaxRegressions: m.MaxRegressions,
			Layer:          layers[m.ID],
			Issues:         issues,
		})
	}
}

// calculateDAGLayers computes the DAG layer for each milestone based on dependencies.
// Layer 0: milestones with no dependencies or empty depends_on
// Layer N: milestones whose dependencies are all in layers < N
func (vm *EpicViewModel) calculateDAGLayers(milestones []client.Milestone) map[int]int {
	layers := make(map[int]int)

	// Iteratively assign layers until all milestones are assigned
	assigned := make(map[int]bool)
	maxIterations := len(milestones) * 2 // prevent infinite loops

	for iteration := 0; iteration < maxIterations && len(assigned) < len(milestones); iteration++ {
		for _, m := range milestones {
			if assigned[m.ID] {
				continue
			}

			// If no dependencies, assign to layer 0
			if len(m.DependsOn) == 0 {
				layers[m.ID] = 0
				assigned[m.ID] = true
				continue
			}

			// Check if all dependencies are assigned
			allDepsAssigned := true
			maxDepLayer := -1
			for _, depID := range m.DependsOn {
				if !assigned[depID] {
					allDepsAssigned = false
					break
				}
				if layers[depID] > maxDepLayer {
					maxDepLayer = layers[depID]
				}
			}

			// If all dependencies assigned, assign this milestone to maxDepLayer + 1
			if allDepsAssigned {
				layers[m.ID] = maxDepLayer + 1
				assigned[m.ID] = true
			}
		}
	}

	return layers
}
