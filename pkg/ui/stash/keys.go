package stash

import tea "github.com/charmbracelet/bubbletea"

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
	kb := m.common.Config.KeyBinds

	switch {
	case kb.Common.Up.Match(key):
		m.moveCursorUp()

	case kb.Common.Down.Match(key):
		m.moveCursorDown()

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

	case kb.Common.Error.Match(key):
		if m.ViewState != StashStateShowingError {
			m.ViewState = StashStateShowingError
		}

	case kb.Common.Escape.Match(key):
		if m.FilterApplied() {
			m.resetFiltering()
		}
	}

	return m, nil
}

// FilterHandler provides filter-specific event handling.
type FilterHandler struct{}

// NewFilterHandler creates a new FilterHandler.
func NewFilterHandler() *FilterHandler {
	return &FilterHandler{}
}

// HandleFilteringMode handles events when in filtering mode.
func (h *FilterHandler) HandleFilteringMode(m *StashModel, msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		filterCmd := h.handleFilterKeys(m, keyMsg.String())
		if filterCmd != nil {
			cmds = append(cmds, filterCmd)
		}
	}

	// Update the filter input component.
	inputCmds := h.updateFilterInput(m, msg)
	cmds = append(cmds, inputCmds...)

	// Update pagination.
	m.updatePagination()

	return cmds
}

// handleFilterKeys handles key events specific to filtering mode.
func (h *FilterHandler) handleFilterKeys(m *StashModel, key string) tea.Cmd {
	switch key {
	case "esc":
		// Cancel filtering.
		m.resetFiltering()

	case "enter", "tab", "shift+tab", "ctrl+k", "up", "ctrl+j", "down":
		if len(m.YAMLs) == 0 {
			return nil
		}

		visibleYAMLs := m.getVisibleYAMLs()

		// If we've filtered down to nothing, clear the filter.
		if len(visibleYAMLs) == 0 {
			m.ViewState = StashStateReady
			m.resetFiltering()

			return nil
		}

		// When there's only one filtered yaml left we can just "open" it directly.
		if len(visibleYAMLs) == 1 {
			m.ViewState = StashStateReady
			m.resetFiltering()

			return m.openYAML(visibleYAMLs[0])
		}

		// Add new section if it's not present.
		if m.sections[len(m.sections)-1].key != filterSection {
			m.sections = append(m.sections, sections[filterSection])
		}
		m.sectionIndex = len(m.sections) - 1

		m.filterInput.Blur()
		m.FilterState = FilterApplied
		if m.filterInput.Value() == "" {
			m.resetFiltering()
		}
	}

	return nil
}

// updateFilterInput updates the filter input component and handles value changes.
func (h *FilterHandler) updateFilterInput(m *StashModel, msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd

	newFilterInputModel, inputCmd := m.filterInput.Update(msg)
	currentFilterVal := m.filterInput.Value()
	newFilterVal := newFilterInputModel.Value()
	m.filterInput = newFilterInputModel
	cmds = append(cmds, inputCmd)

	// If the filtering input has changed, request updated filtering.
	if newFilterVal != currentFilterVal {
		cmds = append(cmds, FilterYAMLs(*m))
	}

	return cmds
}
