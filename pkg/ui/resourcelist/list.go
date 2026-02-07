package resourcelist

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/yamls"
)

const (
	listIndent                    = 1
	listViewTopPadding            = 1
	listViewHorizontalPadding     = 6
	compactExtraHorizontalPadding = 2
)

type FetchedYAMLMsg *yamls.Document

// LoadYAML returns a command that signals a YAML document has been selected.
func LoadYAML(md *yamls.Document) tea.Cmd {
	return func() tea.Msg {
		return FetchedYAMLMsg(md)
	}
}

// Model wraps the bubbles [list.Model] with custom chrome.
type Model struct {
	inner        list.Model
	delegate     *ItemDelegate
	cm           *common.CommonModel
	helpRenderer *statusbar.HelpRenderer
	helpHeight   int
	showHelp     bool
}

// Config holds configuration for creating a new [Model].
type Config struct {
	CommonModel *common.CommonModel
	KeyBinds    *KeyBinds
	Compact     bool
}

// NewModel creates a new [Model].
func NewModel(c Config) Model {
	delegate := NewItemDelegate(c.CommonModel.Theme, c.KeyBinds.Open, c.Compact)

	inner := list.New(nil, delegate, 0, 0)

	// Configure filter input.
	inner.FilterInput.Prompt = "Find:"
	styles := inner.FilterInput.Styles()
	styles.Focused.Prompt = c.CommonModel.Theme.FilterStyle.MarginRight(1)
	styles.Blurred.Prompt = c.CommonModel.Theme.FilterStyle.MarginRight(1)
	styles.Cursor.Color = c.CommonModel.Theme.CursorStyle.GetForeground()
	inner.FilterInput.SetStyles(styles)

	// Map keybindings.
	ckb := c.CommonModel.KeyBinds
	kb := c.KeyBinds

	inner.KeyMap.CursorUp = ckb.Up.BubbleKey()
	inner.KeyMap.CursorDown = ckb.Down.BubbleKey()
	inner.KeyMap.NextPage = ckb.Right.BubbleKey()
	inner.KeyMap.PrevPage = ckb.Left.BubbleKey()
	inner.KeyMap.GoToStart = kb.Home.BubbleKey()
	inner.KeyMap.GoToEnd = kb.End.BubbleKey()
	inner.KeyMap.Filter = kb.Find.BubbleKey()
	inner.KeyMap.ClearFilter = ckb.Escape.BubbleKey()

	// Use enter/arrows/etc. to accept while filtering.
	inner.KeyMap.AcceptWhileFiltering = kb.Open.BubbleKey()
	inner.KeyMap.CancelWhileFiltering = ckb.Escape.BubbleKey()

	// Disable built-in quit (we handle it globally).
	inner.KeyMap.Quit.SetEnabled(false)
	inner.KeyMap.ForceQuit.SetEnabled(false)
	inner.KeyMap.ShowFullHelp.SetEnabled(false)
	inner.KeyMap.CloseFullHelp.SetEnabled(false)

	// Disable built-in chrome (we render our own).
	inner.SetShowTitle(false)
	inner.SetShowStatusBar(false)
	inner.SetShowHelp(false)
	inner.SetShowFilter(false)

	// Style pagination dots to match the theme.
	inner.Styles.ActivePaginationDot = c.CommonModel.Theme.SelectedStyle.SetString("•")
	inner.Styles.InactivePaginationDot = c.CommonModel.Theme.SubtleStyle.SetString("◦")
	inner.Styles.PaginationStyle = lipgloss.NewStyle().PaddingLeft(listIndent).PaddingBottom(1)
	inner.Paginator.ActiveDot = inner.Styles.ActivePaginationDot.String()
	inner.Paginator.InactiveDot = inner.Styles.InactivePaginationDot.String()

	// Infinite scrolling for seamless cursor movement.
	inner.InfiniteScrolling = true

	// Set up help renderer.
	kbr := &keys.KeyBindRenderer{}
	kbr.AddColumn(
		*ckb.Up,
		*ckb.Down,
		*ckb.Left,
		*ckb.Right,
		*ckb.Next,
		*ckb.Prev,
	)
	kbr.AddColumn(
		*kb.Open,
		*kb.Find,
		*kb.PageUp,
		*kb.PageDown,
		*kb.Home,
		*kb.End,
	)
	kbr.AddColumn(
		*ckb.Reload,
		*ckb.Escape,
		*ckb.Error,
		*ckb.Help,
		*ckb.Quit,
		*ckb.Suspend,
	)

	// Add plugin keybinds column if plugins are available.
	_, profile := c.CommonModel.Cmd.GetCurrentProfile()
	if profile != nil {
		pluginBinds := profile.GetPluginKeyBinds()
		if len(pluginBinds) > 6 {
			pluginBinds = pluginBinds[:6]
		}

		kbr.AddColumn(pluginBinds...)
	}

	return Model{
		inner:        inner,
		delegate:     delegate,
		cm:           c.CommonModel,
		helpRenderer: statusbar.NewHelpRenderer(c.CommonModel.Theme, kbr),
	}
}

// Update handles messages for the list model.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if !m.IsFiltering() && m.cm.KeyBinds.Help.Match(msg.String()) {
			m.ToggleHelp()

			return nil
		}

		if m.IsFiltering() {
			if msg.Code == tea.KeyEnter {
				m.inner.SetFilterState(list.FilterApplied)

				// If there's only one item, open it directly.
				if len(m.inner.VisibleItems()) == 1 {
					if doc, ok := m.inner.SelectedItem().(*yamls.Document); ok {
						return LoadYAML(doc)
					}
				}

				return nil
			}

			// Pass printable characters directly to the FilterInput
			// to avoid the list's keymap intercepting special characters.
			// Per bubbletea docs, Text is non-empty only for printable characters.
			if msg.Text != "" {
				var cmd tea.Cmd

				m.inner.FilterInput, cmd = m.inner.FilterInput.Update(msg)

				// Sync the filter text to trigger re-filtering, then restore
				// the Filtering state (SetFilterText sets it to FilterApplied).
				m.inner.SetFilterText(m.inner.FilterInput.Value())
				m.inner.SetFilterState(list.Filtering)

				return cmd
			}
		}
	}

	var cmd tea.Cmd

	m.inner, cmd = m.inner.Update(msg)

	return cmd
}

// View renders the list model.
func (m Model) View() string {
	header := m.headerView()
	listContent := m.documentListView()
	statusBar := m.statusBarView()
	help := m.helpView()

	top := lipgloss.JoinVertical(lipgloss.Top, header, listContent)

	bottomContent := statusBar
	if m.showHelp {
		bottomContent = lipgloss.JoinVertical(lipgloss.Top, statusBar, help)
	}

	availableHeight := max(0, m.cm.Height-lipgloss.Height(top))
	bottom := lipgloss.PlaceVertical(availableHeight, lipgloss.Bottom, bottomContent)

	return lipgloss.JoinVertical(lipgloss.Top, top, bottom)
}

// SetItems sets the documents displayed in the list.
func (m *Model) SetItems(docs []*yamls.Document) tea.Cmd {
	// Sort items.
	slices.SortStableFunc(docs, func(a, b *yamls.Document) int {
		return strings.Compare(
			strings.ToLower(a.Desc+a.Title),
			strings.ToLower(b.Desc+b.Title),
		)
	})

	// Build filter values.
	items := make([]list.Item, 0, len(docs))

	for _, doc := range docs {
		doc.BuildFilterValue()

		items = append(items, doc)
	}

	// Update column widths for compact rendering.
	m.delegate.UpdateColumnWidths(docs)

	return m.inner.SetItems(items)
}

// FilterApplied returns whether a filter is currently active.
func (m Model) FilterApplied() bool {
	return m.inner.FilterState() != list.Unfiltered
}

// FilterState returns the current filter state.
func (m Model) FilterState() list.FilterState {
	return m.inner.FilterState()
}

// IsFiltering returns whether the user is actively typing a filter.
func (m Model) IsFiltering() bool {
	return m.inner.SettingFilter()
}

// ResetFiltering clears the active filter.
func (m *Model) ResetFiltering() {
	m.inner.ResetFilter()
}

// SetSize sets the overall dimensions available to the list.
func (m *Model) SetSize(width, height int) {
	m.cm.Width = width
	m.cm.Height = height

	if m.showHelp && m.helpHeight == 0 {
		m.helpHeight = m.helpRenderer.CalculateHelpHeight()
	}

	// Compute the height available for the inner list (minus our custom chrome).
	chromeHeight := m.chromeHeight()
	contentH := max(1, height-chromeHeight)

	m.inner.SetSize(width, contentH)
}

// GetSelectedYAML returns the currently selected document.
func (m Model) GetSelectedYAML() *yamls.Document {
	item := m.inner.SelectedItem()
	if item == nil {
		return nil
	}

	doc, ok := item.(*yamls.Document)
	if !ok {
		return nil
	}

	return doc
}

// SetHelpVisible sets whether help is displayed.
func (m *Model) SetHelpVisible(visible bool) {
	if visible != m.showHelp {
		m.ToggleHelp()
	}
}

// ToggleHelp toggles the help display.
func (m *Model) ToggleHelp() {
	m.showHelp = !m.showHelp
	m.SetSize(m.cm.Width, m.cm.Height)
}

// IsLoading returns whether the list is still loading.
func (m Model) IsLoading() bool {
	return !m.cm.Loaded
}

// chromeHeight returns the total height consumed by custom chrome elements.
func (m Model) chromeHeight() int {
	helpHeight := 0
	if m.showHelp {
		h := m.helpHeight
		if h == 0 {
			h = m.helpRenderer.CalculateHelpHeight()
		}

		helpHeight = h + 1
	}

	// Header (top padding + content + bottom padding) + status bar + help.
	headerHeight := listViewTopPadding + 1 + 1 // Padding top, content line, padding bottom.
	statusBarHeight := 1

	return headerHeight + statusBarHeight + helpHeight
}

func (m Model) documentListView() string {
	// Show "No results." when filtering yields no matches.
	if m.inner.FilterState() != list.Unfiltered && len(m.inner.VisibleItems()) == 0 {
		return indent(m.cm.Theme.SubtleStyle.Render("No results."), listIndent+2)
	}

	return indent(m.inner.View(), listIndent)
}

func (m Model) helpView() string {
	if m.showHelp {
		return m.helpRenderer.Render(m.cm.Width)
	}

	return ""
}

func (m Model) headerView() string {
	sections, divider := m.getHeaderSections()
	header := strings.Join(sections, divider.String())

	header = lipgloss.NewStyle().
		Padding(listViewTopPadding, listIndent+2, 1).
		Render(header)

	return header
}

func (m Model) getHeaderSections() ([]string, lipgloss.Style) {
	localCount := len(m.inner.Items())

	dividerDot := m.cm.Theme.SubtleStyle.SetString(" • ")
	dividerBar := m.cm.Theme.SubtleStyle.SetString(" │ ")

	// While filtering, show the filter input.
	if m.inner.FilterState() == list.Filtering {
		sections := []string{
			m.cm.Theme.GenericTextStyle.Render(m.inner.FilterInput.View()),
		}

		return sections, dividerDot
	}

	sections := []string{
		m.cm.Theme.SubtleStyle.Render(fmt.Sprintf("%d resources", localCount)),
	}

	// Show filtered count when a filter is applied.
	if m.inner.FilterState() == list.FilterApplied {
		filterSection := fmt.Sprintf(
			"%d %q",
			len(m.inner.VisibleItems()),
			m.inner.FilterValue(),
		)
		sections = append(sections, m.cm.Theme.SelectedStyle.Render(filterSection))
	}

	return sections, dividerBar
}

func (m Model) statusBarView() string {
	title := m.cm.Cmd.String()

	p := m.inner.Paginator
	progress := fmt.Sprintf("%d/%d", p.Page+1, p.TotalPages)

	return m.cm.GetStatusBar().RenderWithNote(title, progress)
}

// StartFiltering starts the filter mode. This is used for programmatic
// triggering from outside the list.
func (m *Model) StartFiltering() tea.Cmd {
	// Build filter values for all items.
	for _, item := range m.inner.Items() {
		if doc, ok := item.(*yamls.Document); ok {
			doc.BuildFilterValue()
		}
	}

	m.inner.SetFilterState(list.Filtering)
	m.inner.FilterInput.Focus()
	m.inner.FilterInput.CursorEnd()

	return textinput.Blink
}

// indent prepends n spaces to each line of s.
func indent(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}

	l := strings.Split(s, "\n")
	b := strings.Builder{}

	prefix := strings.Repeat(" ", n)
	for i, v := range l {
		if i > 0 {
			b.WriteString("\n")
		}

		b.WriteString(prefix)
		b.WriteString(v)
	}

	return b.String()
}

// styleFilteredText applies fuzzy match highlighting to text.
func styleFilteredText(haystack, needles string, defaultStyle, matchedStyle lipgloss.Style) string {
	b := strings.Builder{}

	normalizedHay, err := yamls.Normalize(haystack)
	if err != nil {
		return defaultStyle.Render(haystack)
	}

	matches := fuzzyFind(needles, normalizedHay)
	if len(matches) == 0 {
		return defaultStyle.Render(haystack)
	}

	for i, rune := range []rune(haystack) {
		styled := false
		for _, mi := range matches {
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

// fuzzyFind returns the matched indexes of needles in haystack.
func fuzzyFind(needles, haystack string) []int {
	matches := fuzzy.Find(needles, []string{haystack})
	if len(matches) == 0 {
		return nil
	}

	return matches[0].MatchedIndexes
}
