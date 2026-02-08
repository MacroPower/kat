package resourcelist

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"
	"go.jacobcolvin.com/niceyaml/style"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/theme"
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
	return common.CmdHandler(FetchedYAMLMsg(md))
}

// Model wraps the bubbles [list.Model] with custom chrome.
type Model struct {
	inner         list.Model
	cmd           common.Commander
	delegate      *ItemDelegate
	theme         *theme.Theme
	keyBinds      *common.KeyBinds
	statusBar     *statusbar.StatusBarRenderer
	Help          statusbar.HelpModel
	StatusMessage statusbar.StatusMessageModel
	width         int
	height        int
}

// Config holds configuration for creating a new [Model].
type Config struct {
	Theme     *theme.Theme
	KeyBinds  *KeyBinds
	CKeyBinds *common.KeyBinds
	Cmd       common.Commander
	Compact   bool
}

// NewModel creates a new [Model].
func NewModel(c Config) Model {
	delegate := NewItemDelegate(c.Theme, c.KeyBinds.Open, c.Compact)

	inner := list.New(nil, delegate, 0, 0)

	// Configure filter input.
	inner.FilterInput.Prompt = "Find:"
	styles := inner.FilterInput.Styles()
	styles.Focused.Prompt = c.Theme.Style(style.TextAccentDim).MarginRight(1)
	styles.Blurred.Prompt = c.Theme.Style(style.TextAccentDim).MarginRight(1)
	styles.Cursor.Color = c.Theme.Style(style.TextSubtle).GetForeground()
	inner.FilterInput.SetStyles(styles)

	// Map keybindings.
	ckb := c.CKeyBinds
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
	inner.Styles.ActivePaginationDot = c.Theme.Style(style.TextAccent).SetString("•")
	inner.Styles.InactivePaginationDot = c.Theme.Style(style.TextSubtleDim).SetString("◦")
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
	_, prof := c.Cmd.GetCurrentProfile()
	if prof != nil {
		pluginBinds := prof.GetPluginKeyBinds()
		if len(pluginBinds) > 6 {
			pluginBinds = pluginBinds[:6]
		}

		kbr.AddColumn(pluginBinds...)
	}

	return Model{
		inner:     inner,
		delegate:  delegate,
		theme:     c.Theme,
		keyBinds:  c.CKeyBinds,
		cmd:       c.Cmd,
		Help:      statusbar.NewHelpModel(statusbar.NewHelpRenderer(c.Theme, kbr)),
		statusBar: statusbar.NewStatusBarRenderer(c.Theme, 0),
	}
}

// Update handles messages for the list model.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if !m.IsFiltering() && m.keyBinds.Help.Match(msg.String()) {
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

			// Forward printable characters and text-editing keys directly
			// to the FilterInput to avoid the inner list's keymap
			// intercepting them (e.g. Left/Right bound to PrevPage/NextPage).
			if msg.Text != "" || isTextEditingKey(msg.Code) {
				var cmd tea.Cmd

				m.inner.FilterInput, cmd = m.inner.FilterInput.Update(msg)

				// Save cursor position before syncing, because SetFilterText
				// calls CursorEnd() which would reset it.
				pos := m.inner.FilterInput.Position()

				// Sync the filter text to trigger re-filtering, then restore
				// the Filtering state (SetFilterText sets it to FilterApplied).
				m.inner.SetFilterText(m.inner.FilterInput.Value())
				m.inner.SetFilterState(list.Filtering)

				m.inner.FilterInput.SetCursor(pos)

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
	if m.Help.Visible() {
		bottomContent = lipgloss.JoinVertical(lipgloss.Top, statusBar, help)
	}

	availableHeight := max(0, m.height-lipgloss.Height(top))
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

// IsFiltering returns whether the user is actively typing a filter.
func (m Model) IsFiltering() bool {
	return m.inner.SettingFilter()
}

// ResetFiltering clears the active filter.
func (m *Model) ResetFiltering() {
	m.inner.ResetFilter()
}

// SetSize sets the overall dimensions available to the list.
func (m *Model) SetSize(width, height int) tea.Cmd {
	m.width = width
	m.height = height
	m.Help.SetWidth(width)
	m.statusBar.SetWidth(width)

	// Compute the height available for the inner list (minus our custom chrome).
	chromeHeight := m.chromeHeight()
	contentH := max(1, height-chromeHeight)

	m.inner.SetSize(width, contentH)

	return nil
}

// ClearStatus removes the current status message.
func (m *Model) ClearStatus() {
	m.StatusMessage.Clear()
}

// SetStatusMessage stores a status message and returns a [tea.Cmd] that will
// clear it after the default timeout.
func (m *Model) SetStatusMessage(msg string, s statusbar.Style) tea.Cmd {
	return m.StatusMessage.Set(msg, s)
}

// HandleStatusTimeout handles status message timeout messages. It returns
// true if the message was consumed.
func (m *Model) HandleStatusTimeout(msg tea.Msg) bool {
	return m.StatusMessage.Update(msg)
}

// ToggleHelp toggles the help display.
func (m *Model) ToggleHelp() {
	m.Help.Toggle()
	m.SetSize(m.width, m.height)
}

// chromeHeight returns the total height consumed by custom chrome elements.
func (m Model) chromeHeight() int {
	helpHeight := m.Help.Height()
	if helpHeight > 0 {
		helpHeight++ // Account for separator line between status bar and help.
	}

	// Header (top padding + content + bottom padding) + status bar + help.
	headerHeight := listViewTopPadding + 1 + 1 // Padding top, content line, padding bottom.
	statusBarHeight := 1

	return headerHeight + statusBarHeight + helpHeight
}

func (m Model) documentListView() string {
	// Show "No results." when filtering yields no matches.
	if m.inner.FilterState() != list.Unfiltered && len(m.inner.VisibleItems()) == 0 {
		return indent(m.theme.Style(style.TextSubtleDim).Render("No results."), listIndent+2)
	}

	return indent(m.inner.View(), listIndent)
}

func (m Model) helpView() string {
	return m.Help.View(m.width)
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

	dividerDot := m.theme.Style(style.TextSubtleDim).SetString(" • ")
	dividerBar := m.theme.Style(style.TextSubtleDim).SetString(" │ ")

	// While filtering, show the filter input.
	if m.inner.FilterState() == list.Filtering {
		sections := []string{
			m.theme.Style(style.Text).Render(m.inner.FilterInput.View()),
		}

		return sections, dividerDot
	}

	sections := []string{
		m.theme.Style(style.Text).Render(fmt.Sprintf("%d resources", localCount)),
	}

	// Show filtered count when a filter is applied.
	if m.inner.FilterState() == list.FilterApplied {
		filterSection := fmt.Sprintf(
			"%d %q",
			len(m.inner.VisibleItems()),
			m.inner.FilterValue(),
		)
		sections = append(sections, m.theme.Style(style.TextAccent).Render(filterSection))
	}

	return sections, dividerBar
}

func (m Model) statusBarView() string {
	title := m.cmd.String()

	p := m.inner.Paginator
	progress := fmt.Sprintf("%d/%d", p.Page+1, p.TotalPages)

	var opts []statusbar.StatusBarOpt
	if opt := m.StatusMessage.Opt(); opt != nil {
		opts = append(opts, opt)
	}

	m.statusBar.Apply(opts...)

	return m.statusBar.RenderWithNote(title, progress)
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
// It batches consecutive characters with the same style into runs to minimize
// render calls from O(n) to O(matches+1).
func styleFilteredText(haystack, needles string, defaultStyle, matchedStyle lipgloss.Style) string {
	normalizedHay := yamls.Normalize(haystack)

	matches := fuzzyFind(needles, normalizedHay)
	if len(matches) == 0 {
		return defaultStyle.Render(haystack)
	}

	matchSet := make(map[int]struct{}, len(matches))
	for _, mi := range matches {
		matchSet[mi] = struct{}{}
	}

	runes := []rune(haystack)
	b := strings.Builder{}
	b.Grow(len(haystack) * 2) // Pre-allocate for styled output.

	runStart := 0
	for runStart < len(runes) {
		_, isMatch := matchSet[runStart]
		runEnd := runStart + 1

		for runEnd < len(runes) {
			_, nextMatch := matchSet[runEnd]
			if nextMatch != isMatch {
				break
			}

			runEnd++
		}

		run := string(runes[runStart:runEnd])
		if isMatch {
			b.WriteString(matchedStyle.Render(run))
		} else {
			b.WriteString(defaultStyle.Render(run))
		}

		runStart = runEnd
	}

	return b.String()
}

// isTextEditingKey reports whether the key code is a non-printable key
// handled by textinput for cursor movement and text editing.
func isTextEditingKey(code rune) bool {
	switch code {
	case tea.KeyLeft, tea.KeyRight, tea.KeyHome, tea.KeyEnd,
		tea.KeyBackspace, tea.KeyDelete:
		return true
	}

	return false
}

// fuzzyFind returns the matched indexes of needles in haystack.
func fuzzyFind(needles, haystack string) []int {
	matches := fuzzy.Find(needles, []string{haystack})
	if len(matches) == 0 {
		return nil
	}

	return matches[0].MatchedIndexes
}
