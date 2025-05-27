package stash

// LayoutCalculator helps calculate layout dimensions.
type LayoutCalculator struct {
	totalHeight int
	totalWidth  int
}

// NewLayoutCalculator creates a new layout calculator.
func NewLayoutCalculator(width, height int) *LayoutCalculator {
	return &LayoutCalculator{
		totalWidth:  width,
		totalHeight: height,
	}
}

// CalculateAvailableHeight calculates available height for content.
func (lc *LayoutCalculator) CalculateAvailableHeight(componentHeight ...int) int {
	available := lc.totalHeight
	for _, h := range componentHeight {
		available -= h
	}

	return max(0, available)
}
