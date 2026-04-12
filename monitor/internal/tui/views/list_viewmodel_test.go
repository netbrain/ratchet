package views_test

import (
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

func TestListViewModelInitialState(t *testing.T) {
	var lvm views.ListViewModel
	if lvm.Selected() != 0 {
		t.Errorf("initial Selected = %d, want 0", lvm.Selected())
	}
	if lvm.ScrollOffset() != 0 {
		t.Errorf("initial ScrollOffset = %d, want 0", lvm.ScrollOffset())
	}
}

func TestListViewModelSelectNext(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SelectNext(5)
	if lvm.Selected() != 1 {
		t.Errorf("after SelectNext Selected = %d, want 1", lvm.Selected())
	}
	lvm.SelectNext(5)
	if lvm.Selected() != 2 {
		t.Errorf("after 2x SelectNext Selected = %d, want 2", lvm.Selected())
	}
}

func TestListViewModelSelectNextWraps(t *testing.T) {
	var lvm views.ListViewModel
	for i := 0; i < 5; i++ {
		lvm.SelectNext(5)
	}
	if lvm.Selected() != 0 {
		t.Errorf("SelectNext should wrap to 0, got %d", lvm.Selected())
	}
}

func TestListViewModelSelectNextEmpty(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SelectNext(0) // should not panic
	if lvm.Selected() != 0 {
		t.Errorf("SelectNext on empty list: Selected = %d, want 0", lvm.Selected())
	}
}

func TestListViewModelSelectPrevious(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SelectNext(5) // selected=1
	lvm.SelectNext(5) // selected=2
	lvm.SelectPrevious(5)
	if lvm.Selected() != 1 {
		t.Errorf("after SelectPrevious Selected = %d, want 1", lvm.Selected())
	}
}

func TestListViewModelSelectPreviousWraps(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SelectPrevious(5)
	if lvm.Selected() != 4 {
		t.Errorf("SelectPrevious at 0 should wrap to 4, got %d", lvm.Selected())
	}
}

func TestListViewModelSelectPreviousEmpty(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SelectPrevious(0) // should not panic
	if lvm.Selected() != 0 {
		t.Errorf("SelectPrevious on empty list: Selected = %d, want 0", lvm.Selected())
	}
}

func TestListViewModelSelectFirst(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SelectNext(5)
	lvm.SelectNext(5)
	lvm.SelectNext(5) // selected=3
	lvm.SelectFirst(5)
	if lvm.Selected() != 0 {
		t.Errorf("after SelectFirst Selected = %d, want 0", lvm.Selected())
	}
}

func TestListViewModelSelectLast(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SelectLast(5)
	if lvm.Selected() != 4 {
		t.Errorf("after SelectLast Selected = %d, want 4", lvm.Selected())
	}
}

func TestListViewModelSelectLastEmpty(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SelectLast(0) // should not panic
	if lvm.Selected() != 0 {
		t.Errorf("SelectLast on empty list: Selected = %d, want 0", lvm.Selected())
	}
}

func TestListViewModelClampSelection(t *testing.T) {
	var lvm views.ListViewModel
	// Move to index 4
	for i := 0; i < 4; i++ {
		lvm.SelectNext(5)
	}
	if lvm.Selected() != 4 {
		t.Fatalf("precondition: Selected = %d, want 4", lvm.Selected())
	}

	// Clamp to smaller list
	lvm.ClampSelection(2)
	if lvm.Selected() != 1 {
		t.Errorf("after clamp to 2 items Selected = %d, want 1", lvm.Selected())
	}

	// Clamp to empty list
	lvm.ClampSelection(0)
	if lvm.Selected() != 0 {
		t.Errorf("after clamp to 0 items Selected = %d, want 0", lvm.Selected())
	}
}

func TestListViewModelScrollOffsetFollowsSelection(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SetViewportHeight(2, 5)

	// items 0,1 fit in viewport
	lvm.SelectNext(5) // selected=1
	if lvm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset = %d, want 0", lvm.ScrollOffset())
	}

	// item 2 pushes scroll
	lvm.SelectNext(5) // selected=2
	if lvm.ScrollOffset() != 1 {
		t.Errorf("ScrollOffset = %d, want 1", lvm.ScrollOffset())
	}

	// item 3
	lvm.SelectNext(5) // selected=3
	if lvm.ScrollOffset() != 2 {
		t.Errorf("ScrollOffset = %d, want 2", lvm.ScrollOffset())
	}

	// scroll back up
	lvm.SelectPrevious(5) // selected=2
	lvm.SelectPrevious(5) // selected=1
	if lvm.ScrollOffset() != 1 {
		t.Errorf("ScrollOffset = %d, want 1 after scrolling back", lvm.ScrollOffset())
	}
}

func TestListViewModelScrollOffsetViewportLargerThanList(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SetViewportHeight(20, 5)

	for i := 0; i < 5; i++ {
		lvm.SelectNext(5)
		if lvm.ScrollOffset() != 0 {
			t.Errorf("ScrollOffset = %d, want 0 (viewport larger than list)", lvm.ScrollOffset())
		}
	}
}

func TestListViewModelScrollResetsOnWrapForward(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SetViewportHeight(2, 5)

	// Navigate to last item (index 4)
	for i := 0; i < 4; i++ {
		lvm.SelectNext(5)
	}
	if lvm.Selected() != 4 {
		t.Fatalf("precondition: selected = %d, want 4", lvm.Selected())
	}

	// Wrap forward to 0
	lvm.SelectNext(5)
	if lvm.Selected() != 0 {
		t.Errorf("selected after wrap = %d, want 0", lvm.Selected())
	}
	if lvm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset should reset to 0 on wrap forward, got %d", lvm.ScrollOffset())
	}
}

func TestListViewModelScrollOnWrapBackward(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SetViewportHeight(2, 5)

	// At index 0, wrap backward to last
	lvm.SelectPrevious(5)
	if lvm.Selected() != 4 {
		t.Fatalf("selected = %d, want 4", lvm.Selected())
	}
	if lvm.ScrollOffset() != 3 {
		t.Errorf("ScrollOffset after wrap backward = %d, want 3", lvm.ScrollOffset())
	}
}

func TestListViewModelNegativeViewportHeight(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SetViewportHeight(-5, 5)

	lvm.SelectNext(5)
	lvm.SelectNext(5)
	if lvm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset with negative viewport = %d, want 0", lvm.ScrollOffset())
	}
}

func TestListViewModelZeroViewportHeight(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SetViewportHeight(0, 5)

	lvm.SelectNext(5)
	if lvm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset with zero viewport = %d, want 0", lvm.ScrollOffset())
	}
}

func TestListViewModelViewportResizeResetsOffset(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SetViewportHeight(2, 5)

	// Scroll down
	lvm.SelectNext(5)
	lvm.SelectNext(5)
	lvm.SelectNext(5) // selected=3, offset=2

	// Enlarge viewport to fit everything
	lvm.SetViewportHeight(10, 5)
	if lvm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset = %d, want 0 after viewport enlarge", lvm.ScrollOffset())
	}
}

func TestListViewModelRapidCycling(t *testing.T) {
	var lvm views.ListViewModel
	lvm.SetViewportHeight(2, 4)

	for i := 0; i < 100; i++ {
		lvm.SelectNext(4)
		if lvm.Selected() < 0 || lvm.Selected() >= 4 {
			t.Fatalf("Selected %d out of range after %d SelectNext", lvm.Selected(), i+1)
		}
		if lvm.ScrollOffset() < 0 {
			t.Fatalf("ScrollOffset %d negative after %d SelectNext", lvm.ScrollOffset(), i+1)
		}
	}

	for i := 0; i < 100; i++ {
		lvm.SelectPrevious(4)
		if lvm.Selected() < 0 || lvm.Selected() >= 4 {
			t.Fatalf("Selected %d out of range after %d SelectPrevious", lvm.Selected(), i+1)
		}
	}
}
