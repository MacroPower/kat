package pager

import "strings"

// PagerViewBuilder helps build pager views with proper structure.
type PagerViewBuilder struct {
	components []string
}

// NewPagerViewBuilder creates a new PagerViewBuilder.
func NewPagerViewBuilder() *PagerViewBuilder {
	return &PagerViewBuilder{
		components: make([]string, 0),
	}
}

// AddViewport adds the main viewport content.
func (b *PagerViewBuilder) AddViewport(viewport string) *PagerViewBuilder {
	b.components = append(b.components, viewport+"\n")

	return b
}

// AddStatusBar adds the status bar.
func (b *PagerViewBuilder) AddStatusBar(statusBar string) *PagerViewBuilder {
	b.components = append(b.components, statusBar)

	return b
}

// AddHelp adds the help section if needed.
func (b *PagerViewBuilder) AddHelp(helpView string) *PagerViewBuilder {
	if helpView != "" {
		b.components = append(b.components, "\n"+helpView)
	}

	return b
}

// Build constructs the final pager view.
func (b *PagerViewBuilder) Build() string {
	return strings.Join(b.components, "")
}
