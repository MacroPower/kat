package stash

import (
	"sort"

	"github.com/sahilm/fuzzy"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

// FilterHandler provides filter-specific event handling.
type FilterHandler struct{}

// NewFilterHandler creates a new FilterHandler.
func NewFilterHandler() *FilterHandler {
	return &FilterHandler{}
}

// HandleFilteringMode handles events when in filtering mode.
func (h *FilterHandler) HandleFilteringMode(m StashModel, msg tea.Msg) (StashModel, tea.Cmd) {
	var cmds []tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		var cmd tea.Cmd
		m, cmd = h.handleFilterKeys(m, keyMsg.String())
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Update the filter input component.
	m, inputCmd := h.updateFilterInput(m, msg)
	cmds = append(cmds, inputCmd)

	// Update pagination.
	m.updatePagination()

	return m, tea.Batch(cmds...)
}

// handleFilterKeys handles key events specific to filtering mode.
func (h *FilterHandler) handleFilterKeys(m StashModel, key string) (StashModel, tea.Cmd) {
	kb := m.cm.Config.KeyBinds
	switch {
	case kb.Common.Escape.Match(key):
		// Cancel filtering.
		m.resetFiltering()

		return m, nil

	case kb.Common.Up.Match(key),
		kb.Common.Down.Match(key),
		kb.Common.Next.Match(key),
		kb.Common.Prev.Match(key),
		kb.Stash.Open.Match(key):
		// Apply filter.
		if len(m.YAMLs) == 0 {
			return m, nil
		}

		visibleYAMLs := m.getVisibleYAMLs()

		// If we've filtered down to nothing, clear the filter.
		if len(visibleYAMLs) == 0 {
			m.ViewState = StateReady
			m.resetFiltering()

			return m, nil
		}

		// When there's only one filtered yaml left we can just "open" it directly.
		if len(visibleYAMLs) == 1 {
			m.ViewState = StateReady
			m.resetFiltering()
			cmd := m.openYAML(visibleYAMLs[0])

			return m, cmd
		}

		// Add new section if it's not present.
		if m.sections[len(m.sections)-1].key != SectionFilter {
			m.sections = append(m.sections, Section{
				key:       SectionFilter,
				paginator: newStashPaginator(),
			})
		}
		m.sectionIndex = len(m.sections) - 1

		m.filterInput.Blur()
		m.FilterState = FilterApplied
		if m.filterInput.Value() == "" {
			m.resetFiltering()
		}
	}

	return m, nil
}

// updateFilterInput updates the filter input component and handles value changes.
func (h *FilterHandler) updateFilterInput(m StashModel, msg tea.Msg) (StashModel, tea.Cmd) {
	var cmds []tea.Cmd

	newFilterInputModel, inputCmd := m.filterInput.Update(msg)
	currentFilterVal := m.filterInput.Value()
	newFilterVal := newFilterInputModel.Value()
	m.filterInput = newFilterInputModel
	cmds = append(cmds, inputCmd)

	// If the filtering input has changed, request updated filtering.
	if newFilterVal != currentFilterVal {
		cmds = append(cmds, FilterYAMLs(m))
	}

	return m, tea.Batch(cmds...)
}

func FilterYAMLs(m StashModel) tea.Cmd {
	return func() tea.Msg {
		if m.filterInput.Value() == "" || !m.FilterApplied() {
			return FilteredYAMLMsg(m.YAMLs) // Return everything.
		}

		targets := []string{}
		mds := m.YAMLs

		for _, t := range mds {
			targets = append(targets, t.FilterValue)
		}

		ranks := fuzzy.Find(m.filterInput.Value(), targets)
		sort.Stable(ranks)

		filtered := []*yamldoc.YAMLDocument{}
		for _, r := range ranks {
			filtered = append(filtered, mds[r.Index])
		}

		return FilteredYAMLMsg(filtered)
	}
}
