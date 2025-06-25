package list

import (
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/muesli/reflow/ansi"

	"github.com/MacroPower/kat/pkg/ui/themes"
)

func newListPaginator(theme *themes.Theme) paginator.Model {
	p := paginator.New()
	p.Type = paginator.Dots
	p.ActiveDot = theme.SelectedStyle.Render("•")
	p.InactiveDot = theme.SubtleStyle.Render("◦")
	p.KeyMap = paginator.KeyMap{}

	return p
}

// PaginationRenderer handles pagination display.
type PaginationRenderer struct {
	theme *themes.Theme
	width int
}

// NewPaginationRenderer creates a new pagination renderer.
func NewPaginationRenderer(theme *themes.Theme, width int) *PaginationRenderer {
	return &PaginationRenderer{theme: theme, width: width}
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
