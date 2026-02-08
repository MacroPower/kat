package resourcelist

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"go.jacobcolvin.com/niceyaml/style"

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
	filterValue := m.FilterValue()
	hasEmptyFilter := isFiltering && filterValue == ""
	singleFilteredItem := isFiltering && len(m.VisibleItems()) == 1

	shouldHighlight := (isSelected && !isFiltering) || singleFilteredItem

	if d.compact {
		d.renderCompact(w, doc, filterValue, shouldHighlight, hasEmptyFilter, m.Width())
	} else {
		d.renderNormal(w, doc, filterValue, shouldHighlight, hasEmptyFilter, m.Width())
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

// itemChrome computes the gutter marker and separator string for a list row.
// Highlighted items get a visible bar; others get a blank space.
func (d *ItemDelegate) itemChrome(highlighted bool, unselectedSep string) (string, string) {
	if highlighted {
		return d.theme.Style(style.TextAccent).Render("│"), d.theme.Style(style.TextAccent).Render("")
	}

	return " ", unselectedSep
}

// styleItemText applies filter-aware styling to a single text segment.
// When a filter is active, matched characters are underlined.
// When hasEmptyFilter is true, all text is rendered with the dim style.
func styleItemText(
	text, filterValue string,
	hasEmptyFilter bool,
	baseStyle lipgloss.Style,
) string {
	if hasEmptyFilter {
		return baseStyle.Render(text)
	}

	return styleFilteredText(text, filterValue, baseStyle, baseStyle.Underline(true))
}

func (d *ItemDelegate) renderCompact(
	w io.Writer,
	doc *yamls.Document,
	filterValue string,
	shouldHighlight, hasEmptyFilter bool,
	width int,
) {
	var (
		group = doc.Object.GetGroup()
		kind  = doc.Object.GetKind()
		name  = doc.Title

		horizontalPadding = listViewHorizontalPadding - compactExtraHorizontalPadding

		maxWidth   = float64(d.maxGroupWidth + d.maxKindWidth + d.maxNameWidth)
		groupWidth = int(max(0, float64(width)*float64(float64(d.maxGroupWidth)/maxWidth)))
		kindWidth  = int(max(0, float64(width)*float64(float64(d.maxKindWidth)/maxWidth)))
		nameWidth  = max(0, width-groupWidth-kindWidth-horizontalPadding)
	)

	group = ansi.Truncate(group, groupWidth, d.theme.Ellipsis)
	kind = ansi.Truncate(kind, kindWidth, d.theme.Ellipsis)
	name = ansi.Truncate(name, nameWidth, d.theme.Ellipsis)

	gutter, separator := d.itemChrome(shouldHighlight, "")

	// Compact mode uses a single style for all columns.
	primaryStyle := d.theme.Style(style.TextSubtleDim)
	if shouldHighlight {
		primaryStyle = d.theme.Style(style.TextAccent)
	}

	styledGroup := styleItemText(group, filterValue, hasEmptyFilter, primaryStyle)
	styledKind := styleItemText(kind, filterValue, hasEmptyFilter, primaryStyle)
	styledName := styleItemText(name, filterValue, hasEmptyFilter, primaryStyle)

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
	shouldHighlight, hasEmptyFilter bool,
	width int,
) {
	truncateTo := max(0, width-listViewHorizontalPadding*2)

	title := ansi.Truncate(doc.Title, truncateTo, d.theme.Ellipsis)
	desc := ansi.Truncate(doc.Desc, truncateTo, d.theme.Ellipsis)

	gutter, separator := d.itemChrome(shouldHighlight, d.theme.Style(style.Text).Render(""))

	var titleStyle, descStyle lipgloss.Style

	switch {
	case shouldHighlight:
		titleStyle = d.theme.Style(style.TextAccentDim)
		descStyle = d.theme.Style(style.TextAccent)

	case hasEmptyFilter:
		// Dim both rows when the filter prompt is open but empty.
		titleStyle = d.theme.Style(style.TextSubtleDim)
		descStyle = d.theme.Style(style.TextSubtleDim)

	default:
		titleStyle = d.theme.Style(style.TextSubtle)
		descStyle = d.theme.Style(style.TextSubtleDim)
	}

	styledTitle := styleItemText(title, filterValue, hasEmptyFilter, titleStyle)
	styledDesc := styleItemText(desc, filterValue, hasEmptyFilter, descStyle)

	//nolint:errcheck // Writer is an in-memory buffer.
	fmt.Fprintf(w, "%s %s%s%s\n%s %s", gutter, separator, separator, styledTitle, gutter, styledDesc)
}
