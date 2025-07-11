package list

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
	"github.com/sahilm/fuzzy"

	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/ui/yamls"
)

// DocumentList handles rendering of document lists.
type DocumentList struct {
	theme   *theme.Theme
	indent  int
	compact bool
}

// NewDocumentList creates a new document list.
func NewDocumentList(t *theme.Theme, indent int, compact bool) *DocumentList {
	return &DocumentList{theme: t, indent: indent, compact: compact}
}

func (dlr *DocumentList) GetItemHeight() int {
	if dlr.compact {
		return 1 // Compact mode uses a single line per document.
	}

	return 3
}

// Paginator represents the pagination interface needed for document rendering.
type Paginator interface {
	GetSliceBounds(total int) (start, end int)
}

// DocumentListConfig contains all parameters needed to render a document list.
type DocumentListConfig struct {
	Paginator   Paginator
	FilterValue string
	Documents   []*yamls.Document
	Cursor      int
	Width       int
	FilterState FilterState
	SectionKey  SectionKey
	IsLoaded    bool
}

// Render renders a list of documents with pagination and empty states.
func (dlr *DocumentList) Render(params DocumentListConfig) string {
	var b strings.Builder

	// Handle empty states.
	if len(params.Documents) == 0 {
		f := func(s string) {
			b.WriteString("  " + dlr.theme.SubtleStyle.Render(s))
		}

		switch params.SectionKey {
		case SectionDocuments:
			switch {
			case params.FilterState == Filtering:
				f("No results.")
			case params.IsLoaded:
				f("Nothing to see here.")
			default:
				f("Loading documents...")
			}

		case SectionFilter:
			return ""
		}
	}

	// Render documents with pagination.
	if len(params.Documents) > 0 {
		start, end := params.Paginator.GetSliceBounds(len(params.Documents))
		pageItems := params.Documents[start:end]

		// Compute derived values.
		visibleYAMLsCount := len(params.Documents)
		isFilterSelected := params.SectionKey == SectionFilter

		var maxGroupWidth, maxKindWidth, maxNameWidth int
		for _, doc := range params.Documents {
			group := doc.Object.GetGroup()
			if len(group) > maxGroupWidth {
				maxGroupWidth = len(group)
			}

			kind := doc.Object.GetKind()
			if len(kind) > maxKindWidth {
				maxKindWidth = len(kind)
			}

			if len(doc.Title) > maxNameWidth {
				maxNameWidth = len(doc.Title)
			}
		}

		for i, md := range pageItems {
			item := &DocumentListItem{
				Index:             i,
				Document:          md,
				Theme:             dlr.theme,
				Compact:           dlr.compact,
				Width:             params.Width,
				MaxGroupWidth:     maxGroupWidth,
				MaxKindWidth:      maxKindWidth,
				MaxNameWidth:      maxNameWidth,
				Cursor:            params.Cursor,
				FilterState:       params.FilterState,
				IsFilterSelected:  isFilterSelected,
				VisibleYAMLsCount: visibleYAMLsCount,
				FilterValue:       params.FilterValue,
			}
			item.Render(&b)

			if i != len(pageItems)-1 {
				b.WriteString("\n")
				if !dlr.compact {
					b.WriteString("\n")
				}
			}
		}
	}

	return indent(b.String(), dlr.indent)
}

// DocumentListItem represents a single item in the list with its data and rendering capabilities.
type DocumentListItem struct {
	Document          *yamls.Document
	Theme             *theme.Theme
	FilterValue       string
	Index             int
	Width             int
	MaxGroupWidth     int // Maximum width of the group across all items.
	MaxKindWidth      int // Maximum width of the kind across all items.
	MaxNameWidth      int // Maximum width of the name across all items.
	Cursor            int
	FilterState       FilterState
	VisibleYAMLsCount int
	IsFilterSelected  bool
	Compact           bool
}

// Render renders the list item to the provided string builder.
func (li *DocumentListItem) Render(b *strings.Builder) {
	var (
		// Determine item state.
		isSelected         = li.Index == li.Cursor
		isFiltering        = li.FilterState == Filtering
		isFilterApplied    = li.FilterState == FilterApplied
		isFilterSelected   = li.IsFilterSelected
		singleFilteredItem = isFiltering && li.VisibleYAMLsCount == 1
		filterValue        = li.FilterValue
		hasEmptyFilter     = isFiltering && filterValue == ""

		// If there are multiple items being filtered don't highlight a selected
		// item in the results. If we've filtered down to one item, however,
		// highlight that first item since pressing return will open it.
		shouldHighlight  = (isSelected && !isFiltering) || singleFilteredItem
		shouldShowFilter = (isFilterSelected && isFilterApplied) || singleFilteredItem
	)

	// Determine styling and render directly.
	if li.Compact {
		var (
			gutter, styledGroup, styledKind, styledName, separator string

			group = li.Document.Object.GetGroup()
			kind  = li.Document.Object.GetKind()
			name  = li.Document.Title

			horizontalPadding = listViewHorizontalPadding - compactExtraHorizontalPadding

			maxWidth   = float64(li.MaxGroupWidth + li.MaxKindWidth + li.MaxNameWidth)
			groupWidth = int(max(0, float64(li.Width)*float64((float64(li.MaxGroupWidth)/maxWidth))))
			kindWidth  = int(max(0, float64(li.Width)*float64((float64(li.MaxKindWidth)/maxWidth))))
			nameWidth  = max(0, li.Width-groupWidth-kindWidth-horizontalPadding)

			// Styles.
			selectedStyle = li.Theme.SelectedStyle
			subtleStyle   = li.Theme.SubtleStyle
		)

		group = truncate.StringWithTail(group, uint(groupWidth), li.Theme.Ellipsis) //nolint:gosec // Uses max.
		kind = truncate.StringWithTail(kind, uint(kindWidth), li.Theme.Ellipsis)    //nolint:gosec // Uses max.
		name = truncate.StringWithTail(name, uint(nameWidth), li.Theme.Ellipsis)    //nolint:gosec // Uses max.

		if shouldHighlight {
			// Selected/highlighted styling.
			gutter = selectedStyle.Render("│")
			separator = selectedStyle.Render("")

			if shouldShowFilter {
				styledGroup = styleFilteredText(group, filterValue, selectedStyle, selectedStyle.Underline(true))
				styledKind = styleFilteredText(kind, filterValue, selectedStyle, selectedStyle.Underline(true))
				styledName = styleFilteredText(name, filterValue, selectedStyle, selectedStyle.Underline(true))
			} else {
				styledGroup = selectedStyle.Render(group)
				styledKind = selectedStyle.Render(kind)
				styledName = selectedStyle.Render(name)
			}
		} else {
			// Unselected styling.
			gutter = " "
			separator = ""

			if hasEmptyFilter {
				styledGroup = subtleStyle.Render(group)
				styledKind = subtleStyle.Render(kind)
				styledName = subtleStyle.Render(name)
			} else {
				styledGroup = styleFilteredText(group, filterValue, subtleStyle, subtleStyle.Underline(true))
				styledKind = styleFilteredText(kind, filterValue, subtleStyle, subtleStyle.Underline(true))
				styledName = styleFilteredText(name, filterValue, subtleStyle, subtleStyle.Underline(true))
			}
		}

		styledGroup += strings.Repeat(" ", max(0, min(groupWidth, li.MaxGroupWidth)-len(group)))
		styledKind += strings.Repeat(" ", max(0, min(kindWidth, li.MaxKindWidth)-len(kind)))
		styledName += strings.Repeat(" ", max(0, min(nameWidth, li.MaxNameWidth)-len(name)))

		fmt.Fprintf(b, "%s %s%s%s  %s  %s", gutter, separator, separator, styledGroup, styledKind, styledName)
	} else {
		var (
			gutter, styledTitle, styledDesc, separator string

			// Calculate truncation width based on available space.
			truncateTo = uint(max(0, li.Width-listViewHorizontalPadding*2)) //nolint:gosec // Uses max.

			// Prepare content.
			title = truncate.StringWithTail(li.Document.Title, truncateTo, li.Theme.Ellipsis)
			desc  = truncate.StringWithTail(li.Document.Desc, truncateTo, li.Theme.Ellipsis)
		)
		if shouldHighlight {
			// Selected/highlighted styling.
			gutter = li.Theme.SelectedStyle.Render("│")
			separator = li.Theme.SelectedStyle.Render("")

			if shouldShowFilter {
				styledTitle = styleFilteredText(
					title,
					filterValue,
					li.Theme.SelectedStyle,
					li.Theme.SelectedStyle.Underline(true),
				)
				styledDesc = styleFilteredText(
					desc,
					filterValue,
					li.Theme.SelectedSubtleStyle,
					li.Theme.SelectedSubtleStyle.Underline(true),
				)
			} else {
				styledTitle = li.Theme.SelectedStyle.Render(title)
				styledDesc = li.Theme.SelectedSubtleStyle.Render(desc)
			}
		} else {
			// Unselected styling.
			gutter = " "
			separator = li.Theme.GenericTextStyle.Render("")

			if hasEmptyFilter {
				styledTitle = li.Theme.SubtleStyle.Render(title)
				styledDesc = li.Theme.SubtleStyle.Render(desc)
			} else {
				styledTitle = styleFilteredText(title, filterValue, li.Theme.GenericTextStyle, li.Theme.GenericTextStyle.Underline(true))
				styledDesc = styleFilteredText(desc, filterValue, li.Theme.SubtleStyle, li.Theme.SubtleStyle.Underline(true))
			}
		}

		fmt.Fprintf(b, "%s %s%s%s\n", gutter, separator, separator, styledTitle)
		fmt.Fprintf(b, "%s %s", gutter, styledDesc)
	}
}

func styleFilteredText(haystack, needles string, defaultStyle, matchedStyle lipgloss.Style) string {
	b := strings.Builder{}

	normalizedHay, err := yamls.Normalize(haystack)
	if err != nil {
		slog.Error("error normalizing",
			slog.String("haystack", haystack),
			slog.Any("error", err),
		)
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

// Lightweight version of reflow's indent function.
func indent(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}

	l := strings.Split(s, "\n")
	b := strings.Builder{}

	i := strings.Repeat(" ", n)
	for _, v := range l {
		fmt.Fprintf(&b, "%s%s\n", i, v)
	}

	return b.String()
}
