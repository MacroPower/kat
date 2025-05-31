package stash

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/reflow/truncate"
	"github.com/sahilm/fuzzy"

	"github.com/MacroPower/kat/pkg/ui/styles"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

const (
	verticalLine         = "│"
	fileListingStashIcon = "• "
)

func stashItemView(b *strings.Builder, m StashModel, index int, y *yamldoc.YAMLDocument) {
	var (
		truncateTo  = uint(max(0, m.common.Width-stashViewHorizontalPadding*2)) //nolint:gosec // Uses max.
		gutter      string
		title       = truncate.StringWithTail(y.Title, truncateTo, styles.Ellipsis)
		desc        = truncate.StringWithTail(y.Desc, truncateTo, styles.Ellipsis)
		editedBy    = ""
		hasEditedBy = false
		icon        = ""
		separator   = ""
	)

	isSelected := index == m.cursor()
	isFiltering := m.FilterState == Filtering
	singleFilteredItem := isFiltering && len(m.getVisibleYAMLs()) == 1

	// If there are multiple items being filtered don't highlight a selected
	// item in the results. If we've filtered down to one item, however,
	// highlight that first item since pressing return will open it.
	if isSelected && !isFiltering || singleFilteredItem {
		// Selected item.
		gutter = styles.DullFuchsiaFg(verticalLine)
		if m.currentSection().key == filterSection && m.FilterState == FilterApplied || singleFilteredItem {
			s := lipgloss.NewStyle().Foreground(styles.Fuchsia)
			title = styleFilteredText(title, m.filterInput.Value(), s, s.Underline(true))
		} else {
			title = styles.FuchsiaFg(title)
			icon = styles.FuchsiaFg(icon)
		}
		desc = styles.DimFuchsiaFg(desc)
		editedBy = styles.DimDullFuchsiaFg(editedBy)
		separator = styles.DullFuchsiaFg(separator)
	} else {
		gutter = " "
		if isFiltering && m.filterInput.Value() == "" {
			icon = styles.DimGreenFg(icon)
			title = styles.DimNormalFg(title)
			desc = styles.DimBrightGrayFg(desc)
			editedBy = styles.DimBrightGrayFg(editedBy)
			separator = styles.DimBrightGrayFg(separator)
		} else {
			icon = styles.GreenFg(icon)

			s := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})
			title = styleFilteredText(title, m.filterInput.Value(), s, s.Underline(true))
			desc = styles.GrayFg(desc)
			editedBy = styles.MidGrayFg(editedBy)
			separator = styles.BrightGrayFg(separator)
		}
	}

	fmt.Fprintf(b, "%s %s%s%s%s\n", gutter, icon, separator, separator, title)
	fmt.Fprintf(b, "%s %s", gutter, desc)
	if hasEditedBy {
		fmt.Fprintf(b, " %s", editedBy)
	}
}

func styleFilteredText(haystack, needles string, defaultStyle, matchedStyle lipgloss.Style) string {
	b := strings.Builder{}

	normalizedHay, err := yamldoc.Normalize(haystack)
	if err != nil {
		log.Error("error normalizing", "haystack", haystack, "error", err)
	}

	matches := fuzzy.Find(needles, []string{normalizedHay})
	if len(matches) == 0 {
		return defaultStyle.Render(haystack)
	}

	m := matches[0] // Only one match exists.
	for i, rune := range []rune(haystack) {
		styled := false
		for _, mi := range m.MatchedIndexes {
			if i == mi {
				b.WriteString(matchedStyle.Render(string(rune)))
				styled = true
			}
		}
		if !styled {
			b.WriteString(defaultStyle.Render(string(rune)))
		}
	}

	return b.String()
}
