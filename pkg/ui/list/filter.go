package list

import (
	"sort"

	"github.com/sahilm/fuzzy"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

func FilterYAMLs(m ListModel) tea.Cmd {
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
