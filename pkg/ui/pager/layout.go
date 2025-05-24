package pager

const statusBarHeight = 1

// PagerLayoutCalculator calculates layout dimensions for the pager.
type PagerLayoutCalculator struct {
	width  int
	height int
}

// NewPagerLayoutCalculator creates a new PagerLayoutCalculator.
func NewPagerLayoutCalculator(width, height int) *PagerLayoutCalculator {
	return &PagerLayoutCalculator{
		width:  width,
		height: height,
	}
}

// CalculateViewportHeight calculates the height available for the viewport.
func (c *PagerLayoutCalculator) CalculateViewportHeight(showHelp bool, helpHeight int) int {
	height := c.height - statusBarHeight

	if showHelp {
		height -= (statusBarHeight + helpHeight)
	}

	return height
}

// ValidateScrollPosition ensures the scroll position is within bounds.
func (c *PagerLayoutCalculator) ValidateScrollPosition(pastBottom bool, gotoBottom func() []string) {
	if pastBottom {
		gotoBottom()
	}
}
