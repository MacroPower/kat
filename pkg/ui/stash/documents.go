package stash

import (
	"strings"

	"github.com/MacroPower/kat/pkg/ui/styles"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

// DocumentListRenderer handles rendering of document lists.
type DocumentListRenderer struct {
	width  int
	height int
}

// NewDocumentListRenderer creates a new document list renderer.
func NewDocumentListRenderer(width, height int) *DocumentListRenderer {
	return &DocumentListRenderer{width: width, height: height}
}

// RenderDocumentList renders a list of documents with pagination and empty states.
func (dlr *DocumentListRenderer) RenderDocumentList(docs []*yamldoc.YAMLDocument, m StashModel) string {
	var b strings.Builder

	// Handle empty states.
	if len(docs) == 0 {
		f := func(s string) {
			b.WriteString("  " + styles.GrayFg(s))
		}

		switch m.currentSection().key {
		case documentsSection:
			if m.loadingDone() {
				f("No documents.")
			} else {
				f("Rendering documents...")
			}
		case filterSection:
			return ""
		}
	}

	// Render documents with pagination.
	if len(docs) > 0 {
		start, end := m.paginator().GetSliceBounds(len(docs))
		pageItems := docs[start:end]

		for i, md := range pageItems {
			stashItemView(&b, m, i, md)
			if i != len(pageItems)-1 {
				b.WriteString("\n\n")
			}
		}
	}

	return b.String()
}
