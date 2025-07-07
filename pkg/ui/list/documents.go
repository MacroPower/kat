package list

import (
	"fmt"
	"strings"

	"github.com/macropower/kat/pkg/ui/themes"
	"github.com/macropower/kat/pkg/ui/yamls"
)

// DocumentListRenderer handles rendering of document lists.
type DocumentListRenderer struct {
	theme   *themes.Theme
	width   int
	height  int
	indent  int
	compact bool
}

// NewDocumentListRenderer creates a new document list renderer.
func NewDocumentListRenderer(theme *themes.Theme, indent int, compact bool) *DocumentListRenderer {
	return &DocumentListRenderer{theme: theme, indent: indent, compact: compact}
}

func (dlr *DocumentListRenderer) SetSize(width, height int) {
	dlr.width = width
	dlr.height = height
}

func (dlr *DocumentListRenderer) GetItemHeight() int {
	if dlr.compact {
		return 1 // Compact mode uses a single line per document.
	}

	return 3
}

// RenderDocumentList renders a list of documents with pagination and empty states.
func (dlr *DocumentListRenderer) RenderDocumentList(docs []*yamls.Document, m ListModel) string {
	var b strings.Builder

	// Handle empty states.
	if len(docs) == 0 {
		f := func(s string) {
			b.WriteString("  " + dlr.theme.SubtleStyle.Render(s))
		}

		switch m.currentSection().key {
		case SectionDocuments:
			switch {
			case m.FilterState == Filtering:
				f("No results.")
			case m.cm.Loaded:
				f("Nothing to see here.")
			default:
				f("Loading documents...")
			}
		case SectionFilter:
			return ""
		}
	}

	// Render documents with pagination.
	if len(docs) > 0 {
		start, end := m.paginator().GetSliceBounds(len(docs))
		pageItems := docs[start:end]

		for i, md := range pageItems {
			listItemView(&b, m, i, dlr.compact, md)
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
