package pager

import (
	"fmt"
	"log/slog"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"
	"go.jacobcolvin.com/niceyaml"
	"go.jacobcolvin.com/niceyaml/bubbles/yamlviewport"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/yamls"
)

const statusBarHeight = 1

type ViewState int

const (
	StateReady ViewState = iota
	StateSearching
)

type PagerModel struct {
	cm              *common.CommonModel
	Help            statusbar.HelpModel
	keyHandler      *KeyHandler
	CurrentDocument yamls.Document
	searchInput     textinput.Model
	viewport        yamlviewport.Model
	ViewState       ViewState
}

type Config struct {
	CommonModel *common.CommonModel
	KeyBinds    *KeyBinds
	Printer     *niceyaml.Printer
}

func NewModel(c Config) PagerModel {
	// Init yamlviewport with printer.
	var opts []yamlviewport.Option
	if c.Printer != nil {
		opts = append(opts, yamlviewport.WithPrinter(c.Printer))
	}

	vp := yamlviewport.New(opts...)
	// Disable yamlviewport's built-in KeyMap — kat's key system routes events.
	vp.KeyMap = yamlviewport.KeyMap{}

	kbr := &keys.KeyBindRenderer{}
	ckb := c.CommonModel.KeyBinds
	kb := c.KeyBinds
	kbr.AddColumn(
		*ckb.Up,
		*ckb.Down,
		*kb.PageUp,
		*kb.PageDown,
		*kb.HalfPageUp,
		*kb.HalfPageDown,
	)
	kbr.AddColumn(
		*kb.Copy,
		*kb.Search,
		*kb.NextMatch,
		*kb.PrevMatch,
		*kb.Home,
		*kb.End,
	)
	kbr.AddColumn(
		*kb.ToggleDiffMode,
		*kb.ToggleViewMode,
		*kb.ToggleWordWrap,
		*ckb.Escape,
		*ckb.Help,
		*ckb.Quit,
	)

	// Add plugin keybinds column if plugins are available.
	_, profile := c.CommonModel.Cmd.GetCurrentProfile()
	if profile != nil {
		pluginBinds := profile.GetPluginKeyBinds()
		// Truncate to maximum of 6 plugin keybinds (shown in help).
		if len(pluginBinds) > 6 {
			pluginBinds = pluginBinds[:6]
		}

		kbr.AddColumn(pluginBinds...)
	}

	// Initialize search input.
	si := textinput.New()
	si.Prompt = "Search:"
	styles := si.Styles()
	styles.Focused.Prompt = c.CommonModel.Theme.FilterStyle.MarginRight(1)
	styles.Blurred.Prompt = c.CommonModel.Theme.FilterStyle.MarginRight(1)
	styles.Cursor.Color = c.CommonModel.Theme.CursorStyle.GetForeground()
	si.SetStyles(styles)
	si.Focus()

	m := PagerModel{
		cm:          c.CommonModel,
		keyHandler:  NewKeyHandler(c.KeyBinds, c.CommonModel.KeyBinds),
		Help:        statusbar.NewHelpModel(statusbar.NewHelpRenderer(c.CommonModel.Theme, kbr)),
		ViewState:   StateReady,
		viewport:    vp,
		searchInput: si,
	}

	return m
}

func (m *PagerModel) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Handle search mode.
	if m.ViewState == StateSearching {
		return m.handleSearchMode(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		cmd := m.keyHandler.HandlePagerKeys(m, msg)
		cmds = append(cmds, cmd)

	// We've received terminal dimensions, either for the first time or
	// after a resize.
	case tea.WindowSizeMsg:
		// Size is handled by SetSize called from ui.go.
	}

	// Pass mouse events through to yamlviewport for wheel scrolling.
	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

func (m PagerModel) View() string {
	if m.ViewState == StateSearching {
		return lipgloss.JoinVertical(
			lipgloss.Top,
			m.viewport.View(),
			m.searchBarView(),
			m.helpView(),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Top,
		m.viewport.View(),
		m.statusBarView(),
		m.helpView(),
	)
}

func (m *PagerModel) SetSize(w, h int) {
	// Calculate viewport dimensions.
	viewportHeight := h - statusBarHeight

	// Subtract help height if visible.
	if helpH := m.Help.Height(); helpH > 0 {
		viewportHeight -= (statusBarHeight + helpH)
	}

	m.searchInput.SetWidth(w - len(m.searchInput.Prompt) - ansi.StringWidth(
		m.searchInput.Prompt,
	))

	m.viewport.SetWidth(w)
	m.viewport.SetHeight(viewportHeight)
}

// SetContent sets the YAML content to display.
func (m *PagerModel) SetContent(source *niceyaml.Source) {
	if source == nil || source.IsEmpty() {
		return
	}

	m.viewport.SetTokens(source)
}

// AddRevision adds a new revision for diff tracking.
func (m *PagerModel) AddRevision(source *niceyaml.Source) {
	if source == nil || source.IsEmpty() {
		return
	}

	m.viewport.AddRevision(source)
}

func (m *PagerModel) Unload() {
	slog.Debug("unload pager document")
	if m.Help.Visible() {
		m.ToggleHelp()
	}
	// Clear search state.
	if m.ViewState == StateSearching {
		m.ExitSearch()
	}

	m.viewport.ClearRevisions()
	m.viewport.ClearSearch()

	m.ViewState = StateReady
	m.viewport.GotoTop()
}

func (m *PagerModel) ToggleHelp() {
	m.Help.Toggle()
	m.SetSize(m.cm.Width, m.cm.Height)

	if m.viewport.PastBottom() {
		m.viewport.GotoBottom()
	}
}

func (m PagerModel) statusBarView() string {
	return m.cm.GetStatusBar().RenderWithScroll(m.CurrentDocument.Title, m.viewport.ScrollPercent())
}

func (m PagerModel) helpView() string {
	return m.Help.View(m.cm.Width)
}

// searchBarView renders the search input bar.
func (m PagerModel) searchBarView() string {
	return m.searchInput.View()
}

// StartSearch initializes search mode.
func (m *PagerModel) StartSearch() tea.Cmd {
	m.ViewState = StateSearching

	m.searchInput.Reset()
	m.searchInput.CursorEnd()
	m.searchInput.Focus()

	return textinput.Blink
}

// handleSearchMode handles key events when in search mode.
func (m *PagerModel) handleSearchMode(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()

		switch {
		case m.cm.KeyBinds.Escape.Match(key):
			// Exit search mode.
			m.ExitSearch()

			return nil

		case key == "enter":
			// Apply search and exit search mode.
			searchTerm := m.searchInput.Value()
			if searchTerm != "" {
				m.viewport.SetSearchTerm(searchTerm)
			} else {
				m.viewport.ClearSearch()
			}

			m.ExitSearch()

			// Send status message with match count.
			count := m.viewport.SearchCount()
			if searchTerm != "" && count > 0 {
				idx := m.viewport.SearchIndex()
				statusMsg := fmt.Sprintf("match %d/%d", idx+1, count)
				cmds = append(cmds, m.cm.SendStatusMessage(statusMsg, statusbar.StyleSuccess))
			} else if searchTerm != "" {
				cmds = append(cmds, m.cm.SendStatusMessage("no matches found", statusbar.StyleError))
			}

			return tea.Batch(cmds...)
		}
	}

	// Update search input.
	var cmd tea.Cmd

	m.searchInput, cmd = m.searchInput.Update(msg)
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

// ExitSearch exits search mode.
func (m *PagerModel) ExitSearch() {
	m.ViewState = StateReady
	m.searchInput.Blur()
	m.searchInput.Reset()
}

// MoveUp moves the viewport up.
func (m *PagerModel) MoveUp() {
	m.viewport.ScrollUp(1)
}

// MoveDown moves the viewport down.
func (m *PagerModel) MoveDown() {
	m.viewport.ScrollDown(1)
}

// PageUp moves the viewport up by one page.
func (m *PagerModel) PageUp() {
	m.viewport.PageUp()
}

// PageDown moves the viewport down by one page.
func (m *PagerModel) PageDown() {
	m.viewport.PageDown()
}

// GoToTop moves to the top of the document.
func (m *PagerModel) GoToTop() {
	m.viewport.GotoTop()
}

// GoToBottom moves to the bottom of the document.
func (m *PagerModel) GoToBottom() {
	m.viewport.GotoBottom()
}

// HalfPageUp moves the viewport up by half a page.
func (m *PagerModel) HalfPageUp() {
	m.viewport.HalfPageUp()
}

// HalfPageDown moves the viewport down by half a page.
func (m *PagerModel) HalfPageDown() {
	m.viewport.HalfPageDown()
}

// NextMatch goes to the next search match.
func (m *PagerModel) NextMatch() tea.Cmd {
	count := m.viewport.SearchCount()
	if count == 0 {
		return m.cm.SendStatusMessage("no matches found", statusbar.StyleError)
	}

	m.viewport.SearchNext()

	idx := m.viewport.SearchIndex()
	statusMsg := fmt.Sprintf("match %d/%d", idx+1, count)

	return m.cm.SendStatusMessage(statusMsg, statusbar.StyleSuccess)
}

// PrevMatch goes to the previous search match.
func (m *PagerModel) PrevMatch() tea.Cmd {
	count := m.viewport.SearchCount()
	if count == 0 {
		return m.cm.SendStatusMessage("no matches found", statusbar.StyleError)
	}

	m.viewport.SearchPrevious()

	idx := m.viewport.SearchIndex()
	statusMsg := fmt.Sprintf("match %d/%d", idx+1, count)

	return m.cm.SendStatusMessage(statusMsg, statusbar.StyleSuccess)
}

// CopyContent copies the current document content to clipboard.
func (m *PagerModel) CopyContent() tea.Cmd {
	content := m.CurrentDocument.Body.Content()
	// Copy using OSC 52.
	fmt.Print(ansi.SetSystemClipboard(content))
	// Copy using native system clipboard.
	_ = clipboard.WriteAll(content) //nolint:errcheck // Can be ignored.

	return m.cm.SendStatusMessage("copied contents", statusbar.StyleSuccess)
}

// ToggleDiffMode cycles between diff modes.
func (m *PagerModel) ToggleDiffMode() {
	m.viewport.ToggleDiffMode()
}

// ToggleViewMode cycles between view modes.
func (m *PagerModel) ToggleViewMode() {
	m.viewport.ToggleViewMode()
}

// ToggleWordWrap toggles word wrapping.
func (m *PagerModel) ToggleWordWrap() {
	m.viewport.ToggleWordWrap()
}
