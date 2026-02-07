package statusbar

// HelpModel encapsulates the common help toggling pattern used across views.
// It manages the visibility state, cached height, and rendering of help content.
type HelpModel struct {
	renderer *HelpRenderer
	visible  bool
	height   int // Cached height.
}

// NewHelpModel creates a new [HelpModel] wrapping the given [HelpRenderer].
func NewHelpModel(renderer *HelpRenderer) HelpModel {
	return HelpModel{
		renderer: renderer,
		height:   renderer.CalculateHelpHeight(),
	}
}

// Toggle toggles help visibility.
func (m *HelpModel) Toggle() {
	m.visible = !m.visible
}

// Visible returns whether help is currently visible.
func (m *HelpModel) Visible() bool {
	return m.visible
}

// SetVisible sets help visibility to the given value.
func (m *HelpModel) SetVisible(v bool) {
	m.visible = v
}

// Height returns the cached height consumed by help content.
// Returns 0 when help is not visible.
func (m *HelpModel) Height() int {
	if !m.visible {
		return 0
	}

	return m.height
}

// View renders the help content at the given width.
// Returns an empty string when help is not visible.
func (m *HelpModel) View(width int) string {
	if !m.visible {
		return ""
	}

	return m.renderer.Render(width)
}
