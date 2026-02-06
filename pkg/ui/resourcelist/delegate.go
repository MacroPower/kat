package resourcelist

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/ui/yamls"
)

// ItemDelegate is a custom [list.ItemDelegate] for rendering document items.
type ItemDelegate struct {
	theme   *theme.Theme
	openKey *keys.KeyBind
	compact bool

	maxGroupWidth int
	maxKindWidth  int
	maxNameWidth  int
}

// NewItemDelegate creates a new [ItemDelegate].
func NewItemDelegate(t *theme.Theme, openKey *keys.KeyBind, compact bool) *ItemDelegate {
	return &ItemDelegate{
		theme:   t,
		openKey: openKey,
		compact: compact,
	}
}

// Height returns the height of each list item.
func (d *ItemDelegate) Height() int {
	if d.compact {
		return 1
	}

	return 2
}

// Spacing returns the vertical gap between list items.
func (d *ItemDelegate) Spacing() int {
	if d.compact {
		return 0
	}

	return 1
}

// Update handles messages for the delegate, specifically the "open" key.
func (d *ItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if d.openKey.Match(keyMsg.String()) {
			item := m.SelectedItem()
			if item == nil {
				return nil
			}

			doc, ok := item.(*yamls.Document)
			if !ok {
				return nil
			}

			return LoadYAML(doc)
		}
	}

	return nil
}

// Render renders a single list item.
func (d *ItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	doc, ok := item.(*yamls.Document)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	isFiltering := m.FilterState() == list.Filtering
	isFilterApplied := m.FilterState() == list.FilterApplied
	filterValue := m.FilterValue()
	hasEmptyFilter := isFiltering && filterValue == ""
	singleFilteredItem := isFiltering && len(m.VisibleItems()) == 1

	shouldHighlight := (isSelected && !isFiltering) || singleFilteredItem
	shouldShowFilter := isFilterApplied || singleFilteredItem

	if d.compact {
		d.renderCompact(w, doc, filterValue, shouldHighlight, shouldShowFilter, hasEmptyFilter, m.Width())
	} else {
		d.renderNormal(w, doc, filterValue, shouldHighlight, shouldShowFilter, hasEmptyFilter, m.Width())
	}
}

// UpdateColumnWidths recalculates the maximum column widths across all items.
func (d *ItemDelegate) UpdateColumnWidths(docs []*yamls.Document) {
	d.maxGroupWidth = 0
	d.maxKindWidth = 0
	d.maxNameWidth = 0

	for _, doc := range docs {
		group := doc.Object.GetGroup()
		if len(group) > d.maxGroupWidth {
			d.maxGroupWidth = len(group)
		}

		kind := doc.Object.GetKind()
		if len(kind) > d.maxKindWidth {
			d.maxKindWidth = len(kind)
		}

		if len(doc.Title) > d.maxNameWidth {
			d.maxNameWidth = len(doc.Title)
		}
	}
}

func (d *ItemDelegate) renderCompact(
	w io.Writer,
	doc *yamls.Document,
	filterValue string,
	shouldHighlight, shouldShowFilter, hasEmptyFilter bool,
	width int,
) {
	var (
		gutter, styledGroup, styledKind, styledName, separator string

		group = doc.Object.GetGroup()
		kind  = doc.Object.GetKind()
		name  = doc.Title

		horizontalPadding = listViewHorizontalPadding - compactExtraHorizontalPadding

		maxWidth   = float64(d.maxGroupWidth + d.maxKindWidth + d.maxNameWidth)
		groupWidth = int(max(0, float64(width)*float64(float64(d.maxGroupWidth)/maxWidth)))
		kindWidth  = int(max(0, float64(width)*float64(float64(d.maxKindWidth)/maxWidth)))
		nameWidth  = max(0, width-groupWidth-kindWidth-horizontalPadding)

		selectedStyle = d.theme.SelectedStyle
		subtleStyle   = d.theme.SubtleStyle
	)

	group = ansi.Truncate(group, groupWidth, d.theme.Ellipsis)
	kind = ansi.Truncate(kind, kindWidth, d.theme.Ellipsis)
	name = ansi.Truncate(name, nameWidth, d.theme.Ellipsis)

	if shouldHighlight {
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

	styledGroup += strings.Repeat(" ", max(0, min(groupWidth, d.maxGroupWidth)-len(group)))
	styledKind += strings.Repeat(" ", max(0, min(kindWidth, d.maxKindWidth)-len(kind)))
	styledName += strings.Repeat(" ", max(0, min(nameWidth, d.maxNameWidth)-len(name)))

	//nolint:errcheck // Writer is an in-memory buffer.
	fmt.Fprintf(w, "%s %s%s%s  %s  %s", gutter, separator, separator, styledGroup, styledKind, styledName)
}

func (d *ItemDelegate) renderNormal(
	w io.Writer,
	doc *yamls.Document,
	filterValue string,
	shouldHighlight, shouldShowFilter, hasEmptyFilter bool,
	width int,
) {
	var gutter, styledTitle, styledDesc, separator string

	truncateTo := max(0, width-listViewHorizontalPadding*2)

	title := ansi.Truncate(doc.Title, truncateTo, d.theme.Ellipsis)
	desc := ansi.Truncate(doc.Desc, truncateTo, d.theme.Ellipsis)

	if shouldHighlight {
		gutter = d.theme.SelectedStyle.Render("│")
		separator = d.theme.SelectedStyle.Render("")

		if shouldShowFilter {
			selStyle := d.theme.SelectedStyle
			selSubStyle := d.theme.SelectedSubtleStyle

			styledTitle = styleFilteredText(title, filterValue, selStyle, selStyle.Underline(true))
			styledDesc = styleFilteredText(desc, filterValue, selSubStyle, selSubStyle.Underline(true))
		} else {
			styledTitle = d.theme.SelectedStyle.Render(title)
			styledDesc = d.theme.SelectedSubtleStyle.Render(desc)
		}
	} else {
		gutter = " "
		separator = d.theme.GenericTextStyle.Render("")

		if hasEmptyFilter {
			styledTitle = d.theme.SubtleStyle.Render(title)
			styledDesc = d.theme.SubtleStyle.Render(desc)
		} else {
			genStyle := d.theme.GenericTextStyle
			subStyle := d.theme.SubtleStyle

			styledTitle = styleFilteredText(title, filterValue, genStyle, genStyle.Underline(true))
			styledDesc = styleFilteredText(desc, filterValue, subStyle, subStyle.Underline(true))
		}
	}

	//nolint:errcheck // Writer is an in-memory buffer.
	fmt.Fprintf(w, "%s %s%s%s\n%s %s", gutter, separator, separator, styledTitle, gutter, styledDesc)
}
