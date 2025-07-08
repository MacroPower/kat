package list

import (
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/muesli/reflow/ansi"

	"github.com/macropower/kat/pkg/ui/theme"
)

func newListPaginator(t *theme.Theme) paginator.Model {
	p := paginator.New()
	p.Type = paginator.Dots
	p.ActiveDot = t.SelectedStyle.Render("•")
	p.InactiveDot = t.SubtleStyle.Render("◦")
	p.KeyMap = paginator.KeyMap{}

	return p
}

// PaginationRenderer handles pagination display.
type PaginationRenderer struct {
	theme *theme.Theme
	width int
}

// NewPaginationRenderer creates a new pagination renderer.
func NewPaginationRenderer(t *theme.Theme, width int) *PaginationRenderer {
	return &PaginationRenderer{theme: t, width: width}
}

// RenderPagination renders pagination controls.
func (pr *PaginationRenderer) RenderPagination(paginatorModel *paginator.Model, totalPages int) string {
	if totalPages <= 1 {
		return ""
	}

	pagination := paginatorModel.View()

	// If the dot pagination is wider than available space, use arabic numerals.
	availableWidth := pr.width - listViewHorizontalPadding
	if ansi.PrintableRuneWidth(pagination) > availableWidth {
		// Create a copy to avoid mutating the original.
		p := *paginatorModel
		p.Type = paginator.Arabic
		pagination = p.View()
	}

	return pr.theme.PaginationStyle.
		PaddingLeft(2).
		PaddingBottom(1).
		Render(pagination)
}
