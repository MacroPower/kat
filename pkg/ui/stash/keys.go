package stash

import (
	tea "github.com/charmbracelet/bubbletea"
)

// StashKeyHandler provides key handling for stash view.
type StashKeyHandler struct{}

// NewStashKeyHandler creates a new StashKeyHandler.
func NewStashKeyHandler() *StashKeyHandler {
	return &StashKeyHandler{}
}

// HandleDocumentBrowsing handles key events for document browsing in stash view.
func (h *StashKeyHandler) HandleDocumentBrowsing(m StashModel, msg tea.KeyMsg) (StashModel, tea.Cmd) {
	key := msg.String()

	// Handle navigation keys.
	numDocs := len(m.getVisibleYAMLs())
	kb := m.cm.Config.KeyBinds

	switch {
	case kb.Common.Up.Match(key):
		m.moveCursorUp()

	case kb.Common.Down.Match(key):
		m.moveCursorDown()

	case kb.Stash.PageUp.Match(key):
		m.setCursor(0)

	case kb.Stash.PageDown.Match(key):
		m.setCursor(m.paginator().ItemsOnPage(numDocs) - 1)

	case kb.Common.Left.Match(key), kb.Common.Prev.Match(key):
		if len(m.sections) == 0 || m.FilterState == Filtering {
			return m, nil
		}
		newPaginatorModel, cmd := m.paginator().Update(msg)
		m.setPaginator(newPaginatorModel)
		m.paginator().PrevPage()
		m.enforcePaginationBounds()

		return m, cmd

	case kb.Common.Right.Match(key), kb.Common.Next.Match(key):
		if len(m.sections) == 0 || m.FilterState == Filtering {
			return m, nil
		}
		newPaginatorModel, cmd := m.paginator().Update(msg)
		m.setPaginator(newPaginatorModel)
		m.paginator().NextPage()
		m.enforcePaginationBounds()

		return m, cmd

	case kb.Stash.Home.Match(key):
		m.paginator().Page = 0
		m.setCursor(0)

	case kb.Stash.End.Match(key):
		m.paginator().Page = m.paginator().TotalPages - 1
		m.setCursor(m.paginator().ItemsOnPage(numDocs) - 1)

	// Document actions.
	case kb.Stash.Open.Match(key):
		if numDocs != 0 {
			md := m.selectedYAML()
			cmd := m.openYAML(md)

			return m, cmd
		}

	// Filtering actions.
	case kb.Stash.Find.Match(key):
		cmd := m.startFiltering()

		return m, cmd

	// Other actions.
	case kb.Common.Help.Match(key):
		m.toggleHelp()
	}

	return m, nil
}
