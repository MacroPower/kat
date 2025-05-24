package stash

import tea "github.com/charmbracelet/bubbletea"

// StashKeyHandler provides key handling for stash view.
type StashKeyHandler struct{}

// NewStashKeyHandler creates a new StashKeyHandler.
func NewStashKeyHandler() *StashKeyHandler {
	return &StashKeyHandler{}
}

// HandleDocumentBrowsing handles key events for document browsing in stash view.
func (h *StashKeyHandler) HandleDocumentBrowsing(m *StashModel, msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd
	key := msg.String()

	// Handle navigation keys.
	m.handleNavigationKeys(key)

	// Handle document-specific keys.
	docCmd := m.handleDocumentKeys(key)
	if docCmd != nil {
		cmds = append(cmds, docCmd)
	}

	// Handle filter keys.
	filterCmd := m.handleFilterKeys(key)
	if filterCmd != nil {
		cmds = append(cmds, filterCmd)
	}

	// Handle section navigation.
	h.handleSectionNavigation(m, key)

	// Handle special actions.
	h.handleSpecialActions(m, key)

	return cmds
}

// handleSectionNavigation handles tab navigation between sections.
func (h *StashKeyHandler) handleSectionNavigation(m *StashModel, key string) {
	switch key {
	case "tab", "L":
		if len(m.sections) == 0 || m.FilterState == Filtering {
			return
		}
		m.sectionIndex++
		if m.sectionIndex >= len(m.sections) {
			m.sectionIndex = 0
		}
		m.updatePagination()

	case "shift+tab", "H":
		if len(m.sections) == 0 || m.FilterState == Filtering {
			return
		}
		m.sectionIndex--
		if m.sectionIndex < 0 {
			m.sectionIndex = len(m.sections) - 1
		}
		m.updatePagination()
	}
}

// handleSpecialActions handles special action keys like refresh, help, etc.
func (h *StashKeyHandler) handleSpecialActions(m *StashModel, key string) {
	switch key {
	case "?":
		m.showFullHelp = !m.showFullHelp
		m.updatePagination()

	case "!":
		if m.err != nil && m.ViewState == StashStateReady {
			m.ViewState = StashStateShowingError
		}

	case "esc":
		if m.FilterApplied() {
			m.resetFiltering()
		}
	}
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
		m.hideStatusMessage()

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

// PaginationKeyHandler handles pagination-specific key events.
type PaginationKeyHandler struct{}

// NewPaginationKeyHandler creates a new PaginationKeyHandler.
func NewPaginationKeyHandler() *PaginationKeyHandler {
	return &PaginationKeyHandler{}
}

// HandlePaginationKeys handles pagination key events.
func (h *PaginationKeyHandler) HandlePaginationKeys(m *StashModel, msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd

	// Update the standard paginator.
	newPaginatorModel, cmd := m.paginator().Update(msg)
	m.setPaginator(newPaginatorModel)
	cmds = append(cmds, cmd)

	// Handle extra pagination keystrokes.
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "b", "u":
			m.paginator().PrevPage()
		case "f", "d":
			m.paginator().NextPage()
		}
	}

	// Keep the index in bounds when paginating.
	h.enforcePaginationBounds(m)

	return cmds
}

// enforcePaginationBounds ensures cursor stays within page bounds.
func (h *PaginationKeyHandler) enforcePaginationBounds(m *StashModel) {
	itemsOnPage := m.paginator().ItemsOnPage(len(m.getVisibleYAMLs()))
	if m.cursor() > itemsOnPage-1 {
		m.setCursor(max(0, itemsOnPage-1))
	}
}
