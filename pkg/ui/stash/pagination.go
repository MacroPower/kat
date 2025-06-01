package stash

import (
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/muesli/reflow/ansi"

	"github.com/MacroPower/kat/pkg/ui/styles"
)

func newStashPaginator() paginator.Model {
	p := paginator.New()
	p.Type = paginator.Dots
	p.ActiveDot = styles.FuchsiaFg("•")
	p.InactiveDot = styles.GrayFg("◦")

	return p
}

// PaginationRenderer handles pagination display.
type PaginationRenderer struct {
	width int
}

// NewPaginationRenderer creates a new pagination renderer.
func NewPaginationRenderer(width int) *PaginationRenderer {
	return &PaginationRenderer{width: width}
}

// RenderPagination renders pagination controls.
func (pr *PaginationRenderer) RenderPagination(paginatorModel *paginator.Model, totalPages int) string {
	if totalPages <= 1 {
		return ""
	}

	pagination := paginatorModel.View()

	// If the dot pagination is wider than available space, use arabic numerals.
	availableWidth := pr.width - stashViewHorizontalPadding
	if ansi.PrintableRuneWidth(pagination) > availableWidth {
		// Create a copy to avoid mutating the original.
		p := *paginatorModel
		p.Type = paginator.Arabic
		pagination = p.View()
	}

	return styles.PaginationStyle.Render(pagination)
}
