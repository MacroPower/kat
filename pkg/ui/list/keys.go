package list

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/themes"
)

type KeyBinds struct {
	Open     *keys.KeyBind `json:"open,omitempty"`
	Find     *keys.KeyBind `json:"find,omitempty"`
	Home     *keys.KeyBind `json:"home,omitempty"`
	End      *keys.KeyBind `json:"end,omitempty"`
	PageUp   *keys.KeyBind `json:"pageUp,omitempty"`
	PageDown *keys.KeyBind `json:"pageDown,omitempty"`
}

func (kb *KeyBinds) EnsureDefaults() {
	keys.SetDefaultBind(&kb.Open,
		keys.NewBind("open",
			keys.New("enter", keys.WithAlias("↵")),
		))
	keys.SetDefaultBind(&kb.Find,
		keys.NewBind("find",
			keys.New("/"),
		))
	keys.SetDefaultBind(&kb.Home,
		keys.NewBind("go to start",
			keys.New("home"),
			keys.New("g"),
		))
	keys.SetDefaultBind(&kb.End,
		keys.NewBind("go to end",
			keys.New("end"),
			keys.New("G"),
		))
	keys.SetDefaultBind(&kb.PageUp,
		keys.NewBind("page up",
			keys.New("pgup"),
			keys.New("b"),
			keys.New("u"),
		))
	keys.SetDefaultBind(&kb.PageDown,
		keys.NewBind("page down",
			keys.New("pgdown", keys.WithAlias("pgdn")),
			keys.New("f"),
			keys.New("d"),
		))
}

func (kb *KeyBinds) GetKeyBinds() []keys.KeyBind {
	return []keys.KeyBind{
		*kb.Open,
		*kb.Find,
		*kb.Home,
		*kb.End,
		*kb.PageUp,
		*kb.PageDown,
	}
}

// KeyHandler provides key handling for list view.
type KeyHandler struct {
	kb    *KeyBinds
	ckb   *common.KeyBinds
	theme *themes.Theme // TODO: Remove this dependency.
}

// NewKeyHandler creates a new ListKeyHandler.
func NewKeyHandler(kb *KeyBinds, ckb *common.KeyBinds, theme *themes.Theme) *KeyHandler {
	return &KeyHandler{
		kb:    kb,
		ckb:   ckb,
		theme: theme,
	}
}

// HandleDocumentBrowsing handles key events for document browsing in list view.
func (h *KeyHandler) HandleDocumentBrowsing(m ListModel, msg tea.KeyMsg) (ListModel, tea.Cmd) {
	key := msg.String()

	// Handle navigation keys.
	numDocs := len(m.getVisibleYAMLs())
	switch {
	case h.ckb.Up.Match(key):
		m.moveCursorUp()

	case h.ckb.Down.Match(key):
		m.moveCursorDown()

	case h.kb.PageUp.Match(key):
		m.setCursor(0)

	case h.kb.PageDown.Match(key):
		m.setCursor(m.paginator().ItemsOnPage(numDocs) - 1)

	case h.ckb.Left.Match(key), h.ckb.Prev.Match(key):
		if len(m.sections) == 0 || m.FilterState == Filtering {
			return m, nil
		}
		newPaginatorModel, cmd := m.paginator().Update(msg)
		m.setPaginator(newPaginatorModel)
		m.paginator().PrevPage()
		m.enforcePaginationBounds()

		return m, cmd

	case h.ckb.Right.Match(key), h.ckb.Next.Match(key):
		if len(m.sections) == 0 || m.FilterState == Filtering {
			return m, nil
		}
		newPaginatorModel, cmd := m.paginator().Update(msg)
		m.setPaginator(newPaginatorModel)
		m.paginator().NextPage()
		m.enforcePaginationBounds()

		return m, cmd

	case h.kb.Home.Match(key):
		m.paginator().Page = 0
		m.setCursor(0)

	case h.kb.End.Match(key):
		m.paginator().Page = m.paginator().TotalPages - 1
		m.setCursor(m.paginator().ItemsOnPage(numDocs) - 1)

	// Document actions.
	case h.kb.Open.Match(key):
		if numDocs != 0 {
			md := m.selectedYAML()
			cmd := m.openYAML(md)

			return m, cmd
		}

	// Filtering actions.
	case h.kb.Find.Match(key):
		cmd := m.startFiltering()

		return m, cmd

	// Other actions.
	case h.ckb.Help.Match(key):
		m.toggleHelp()
	}

	return m, nil
}

// HandleFilteringMode handles events when in filtering mode.
func (h *KeyHandler) HandleFilteringMode(m ListModel, msg tea.Msg) (ListModel, tea.Cmd) {
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
func (h *KeyHandler) handleFilterKeys(m ListModel, key string) (ListModel, tea.Cmd) {
	switch {
	case h.ckb.Up.Match(key),
		h.ckb.Down.Match(key),
		h.ckb.Next.Match(key),
		h.ckb.Prev.Match(key),
		h.kb.Open.Match(key):
		// Apply filter.
		if len(m.YAMLs) == 0 {
			return m, nil
		}

		visibleYAMLs := m.getVisibleYAMLs()

		// If we've filtered down to nothing, clear the filter.
		if len(visibleYAMLs) == 0 {
			m.ResetFiltering()

			return m, nil
		}

		// When there's only one filtered yaml left we can just "open" it directly.
		if len(visibleYAMLs) == 1 {
			m.ResetFiltering()
			cmd := m.openYAML(visibleYAMLs[0])

			return m, cmd
		}

		// Add new section if it's not present.
		if m.sections[len(m.sections)-1].key != SectionFilter {
			m.sections = append(m.sections, Section{
				key:       SectionFilter,
				paginator: newListPaginator(h.theme),
			})
		}
		m.sectionIndex = len(m.sections) - 1

		m.filterInput.Blur()
		m.FilterState = FilterApplied
		if m.filterInput.Value() == "" {
			m.ResetFiltering()
		}
	}

	return m, nil
}

// updateFilterInput updates the filter input component and handles value changes.
func (h *KeyHandler) updateFilterInput(m ListModel, msg tea.Msg) (ListModel, tea.Cmd) {
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
