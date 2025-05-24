package pager

// PagerKeyHandler handles key events specific to the pager.
type PagerKeyHandler struct{}

// NewPagerKeyHandler creates a new PagerKeyHandler.
func NewPagerKeyHandler() *PagerKeyHandler {
	return &PagerKeyHandler{}
}

// HandlePagerKeys handles pager-specific key events.
func (h *PagerKeyHandler) HandlePagerKeys(m PagerModel, key string) (PagerModel, bool) {
	switch key {
	case "home", "g":
		m.viewport.GotoTop()

		return m, true

	case "end", "G":
		m.viewport.GotoBottom()

		return m, true

	case "d":
		m.viewport.HalfPageDown()

		return m, true

	case "u":
		m.viewport.HalfPageUp()

		return m, true

	case "?":
		m.toggleHelp()

		return m, true
	}

	return m, false
}
