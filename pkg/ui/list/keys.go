package list

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ListKeyHandler provides key handling for list view.
type ListKeyHandler struct{}

// NewListKeyHandler creates a new ListKeyHandler.
func NewListKeyHandler() *ListKeyHandler {
	return &ListKeyHandler{}
}

// HandleDocumentBrowsing handles key events for document browsing in list view.
func (h *ListKeyHandler) HandleDocumentBrowsing(m ListModel, msg tea.KeyMsg) (ListModel, tea.Cmd) {
	key := msg.String()

	// Handle navigation keys.
	numDocs := len(m.getVisibleYAMLs())
	kb := m.cm.Config.KeyBinds

	switch {
	case kb.Common.Up.Match(key):
		m.moveCursorUp()

	case kb.Common.Down.Match(key):
		m.moveCursorDown()

	case kb.List.PageUp.Match(key):
		m.setCursor(0)

	case kb.List.PageDown.Match(key):
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

	case kb.List.Home.Match(key):
		m.paginator().Page = 0
		m.setCursor(0)

	case kb.List.End.Match(key):
		m.paginator().Page = m.paginator().TotalPages - 1
		m.setCursor(m.paginator().ItemsOnPage(numDocs) - 1)

	// Document actions.
	case kb.List.Open.Match(key):
		if numDocs != 0 {
			md := m.selectedYAML()
			cmd := m.openYAML(md)

			return m, cmd
		}

	// Filtering actions.
	case kb.List.Find.Match(key):
		cmd := m.startFiltering()

		return m, cmd

	// Other actions.
	case kb.Common.Help.Match(key):
		m.toggleHelp()
	}

	return m, nil
}
