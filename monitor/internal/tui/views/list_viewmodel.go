package views

// ListViewModel provides shared selection and scrolling logic for list-based
// view models. Embed this struct and call its methods, passing the current
// item count, to eliminate duplicated navigation code.
type ListViewModel struct {
	selected       int
	scrollOffset   int
	viewportHeight int
}

// Selected returns the current selection index.
func (l *ListViewModel) Selected() int { return l.selected }

// ScrollOffset returns the current scroll offset.
func (l *ListViewModel) ScrollOffset() int { return l.scrollOffset }

// SetViewportHeight sets the viewport height and recalculates scroll offset.
// The caller must supply the current item count.
func (l *ListViewModel) SetViewportHeight(h, itemCount int) {
	l.viewportHeight = h
	l.AdjustScrollOffset(itemCount)
}

// SelectNext moves selection forward with wrap-around.
func (l *ListViewModel) SelectNext(itemCount int) {
	if itemCount == 0 {
		return
	}
	l.selected = (l.selected + 1) % itemCount
	l.AdjustScrollOffset(itemCount)
}

// SelectPrevious moves selection backward with wrap-around.
func (l *ListViewModel) SelectPrevious(itemCount int) {
	if itemCount == 0 {
		return
	}
	l.selected = (l.selected - 1 + itemCount) % itemCount
	l.AdjustScrollOffset(itemCount)
}

// SelectFirst jumps to the first item.
func (l *ListViewModel) SelectFirst(itemCount int) {
	l.selected = 0
	l.AdjustScrollOffset(itemCount)
}

// SelectLast jumps to the last item.
func (l *ListViewModel) SelectLast(itemCount int) {
	if itemCount == 0 {
		return
	}
	l.selected = itemCount - 1
	l.AdjustScrollOffset(itemCount)
}

// ClampSelection ensures the selection index is within [0, itemCount).
// If itemCount is 0, selection is reset to 0.
func (l *ListViewModel) ClampSelection(itemCount int) {
	if itemCount == 0 {
		l.selected = 0
	} else if l.selected >= itemCount {
		l.selected = itemCount - 1
	}
	l.AdjustScrollOffset(itemCount)
}

// AdjustScrollOffset ensures the selected item is visible within the viewport.
func (l *ListViewModel) AdjustScrollOffset(itemCount int) {
	if l.viewportHeight <= 0 || itemCount <= l.viewportHeight {
		l.scrollOffset = 0
		return
	}
	if l.selected < l.scrollOffset {
		l.scrollOffset = l.selected
	}
	if l.selected >= l.scrollOffset+l.viewportHeight {
		l.scrollOffset = l.selected - l.viewportHeight + 1
	}
}
