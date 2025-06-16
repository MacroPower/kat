package stash

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/reflow/truncate"
	"github.com/sahilm/fuzzy"

	"github.com/MacroPower/kat/pkg/ui/themes"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

// stashItemDisplayState represents the visual state of a stash item.
type stashItemDisplayState struct {
	gutter    string
	title     string
	desc      string
	separator string
}

func stashItemView(b *strings.Builder, m StashModel, index int, compact bool, y *yamldoc.YAMLDocument) {
	var (
		// Calculate truncation width based on available space.
		truncateTo = uint(max(0, m.cm.Width-stashViewHorizontalPadding*2)) //nolint:gosec // Uses max.

		// Prepare content.
		title = truncate.StringWithTail(y.Title, truncateTo, m.cm.Theme.Ellipsis)
		desc  = truncate.StringWithTail(y.Desc, truncateTo, m.cm.Theme.Ellipsis)

		// Determine item state.
		isSelected         = index == m.cursor()
		isFiltering        = m.FilterState == Filtering
		isFilterApplied    = m.FilterState == FilterApplied
		isFilterSelected   = m.currentSection().key == SectionFilter
		singleFilteredItem = isFiltering && len(m.getVisibleYAMLs()) == 1
		filterValue        = m.filterInput.Value()

		// If there are multiple items being filtered don't highlight a selected
		// item in the results. If we've filtered down to one item, however,
		// highlight that first item since pressing return will open it.
		shouldHighlight  = (isSelected && !isFiltering) || singleFilteredItem
		shouldShowFilter = (isFilterSelected && isFilterApplied) || singleFilteredItem
	)

	// Apply appropriate styling based on state.
	var displayState stashItemDisplayState
	if shouldHighlight {
		displayState = applySelectedStyling(m.cm.Theme, title, desc, shouldShowFilter, filterValue)
	} else {
		displayState = applyUnselectedStyling(m.cm.Theme, title, desc, isFiltering, filterValue)
	}

	// Render the item.
	if compact {
		renderStashItemCompact(b, displayState)
	} else {
		renderStashItem(b, displayState)
	}
}

// applySelectedStyling applies styling for selected/highlighted items.
func applySelectedStyling(theme *themes.Theme, title, desc string, showFilter bool, filterValue string) stashItemDisplayState {
	result := stashItemDisplayState{
		gutter:    theme.SelectedStyle.Render("│"),
		desc:      theme.SelectedSubtleStyle.Render(desc),
		separator: theme.SelectedStyle.Render(""),
	}

	if showFilter {
		// Apply filtered text styling.
		result.title = styleFilteredText(title, filterValue, theme.SelectedStyle, theme.SelectedStyle.Underline(true))
	} else {
		// Apply standard selected styling.
		result.title = theme.SelectedStyle.Render(title)
	}

	return result
}

// applyUnselectedStyling applies styling for unselected items.
func applyUnselectedStyling(theme *themes.Theme, title, desc string, isFiltering bool, filterValue string) stashItemDisplayState {
	hasEmptyFilter := isFiltering && filterValue == ""

	result := stashItemDisplayState{
		gutter: " ",
	}

	if hasEmptyFilter {
		// Dimmed styling when filtering with empty input.
		result.title = theme.SubtleStyle.Render(title)
		result.desc = theme.SubtleStyle.Render(desc)
		result.separator = theme.GenericTextStyle.Render("")
	} else {
		// Normal unselected styling.
		result.desc = theme.GenericTextStyle.Render(desc)
		result.separator = theme.GenericTextStyle.Render("")

		// Apply filtered text styling.
		result.title = styleFilteredText(title, filterValue, theme.GenericTextStyle, theme.GenericTextStyle.Underline(true))
		result.desc = styleFilteredText(desc, filterValue, theme.SubtleStyle, theme.SubtleStyle.Underline(true))
	}

	return result
}

// renderStashItemCompact renders the final output for a stash item.
func renderStashItemCompact(b *strings.Builder, state stashItemDisplayState) {
	fmt.Fprintf(b, "%s %s%s%s %s", state.gutter, state.separator, state.separator, state.desc, state.title)
}

// renderStashItem renders the final output for a stash item.
func renderStashItem(b *strings.Builder, state stashItemDisplayState) {
	fmt.Fprintf(b, "%s %s%s%s\n", state.gutter, state.separator, state.separator, state.title)
	fmt.Fprintf(b, "%s %s", state.gutter, state.desc)
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
