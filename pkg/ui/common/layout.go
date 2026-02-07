package common

import tea "charm.land/bubbletea/v2"

// Sizeable is implemented by components that accept a width and height.
// Returning a [tea.Cmd] allows resize-triggered re-renders.
type Sizeable interface {
	SetSize(width, height int) tea.Cmd
}

// Focusable is implemented by components that can receive and release focus.
type Focusable interface {
	Focus() tea.Cmd
	Blur() tea.Cmd
	IsFocused() bool
}
