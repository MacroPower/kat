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
	"go.jacobcolvin.com/niceyaml/style"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/ui/yamls"
)

const statusBarHeight = 1

type ViewState int

const (
	StateReady ViewState = iota
	StateSearching
)

// LoadDocumentMsg instructs the pager to display a new document.
type LoadDocumentMsg struct {
	Document yamls.Document
}

// RevisionMsg instructs the pager to add a revision for diff tracking.
type RevisionMsg struct {
	Document yamls.Document
}

// ExitSearchMsg instructs the pager to exit search mode.
type ExitSearchMsg struct{}

type Model struct {
	keyBinds        *common.KeyBinds
	keyHandler      *KeyHandler
	CurrentDocument yamls.Document
	StatusMessage   statusbar.StatusMessageModel
	Help            statusbar.HelpModel
	statusBar       *statusbar.StatusBarRenderer
	searchInput     textinput.Model
	viewport        yamlviewport.Model
	height          int
	width           int
	ViewState       ViewState
	showingResult   bool
}

type Config struct {
	Theme     *theme.Theme
	KeyBinds  *KeyBinds
	CKeyBinds *common.KeyBinds
	Cmd       common.Commander
	Printer   *niceyaml.Printer
}

func NewModel(c Config) Model {
	// Init yamlviewport with printer.
	var opts []yamlviewport.Option
	if c.Printer != nil {
		opts = append(opts, yamlviewport.WithPrinter(c.Printer))
	}

	vp := yamlviewport.New(opts...)
	// Disable yamlviewport's built-in KeyMap — kat's key system routes events.
	vp.KeyMap = yamlviewport.KeyMap{}

	kbr := &keys.KeyBindRenderer{}
	ckb := c.CKeyBinds
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
	_, prof := c.Cmd.GetCurrentProfile()
	if prof != nil {
		pluginBinds := prof.GetPluginKeyBinds()
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
	styles.Focused.Prompt = c.Theme.Style(style.TextAccentDim).MarginRight(1)
	styles.Blurred.Prompt = c.Theme.Style(style.TextAccentDim).MarginRight(1)
	styles.Cursor.Color = c.Theme.Style(style.TextSubtle).GetForeground()
	si.SetStyles(styles)
	si.Focus()

	m := Model{
		keyBinds:    c.CKeyBinds,
		keyHandler:  NewKeyHandler(c.KeyBinds, c.CKeyBinds),
		Help:        statusbar.NewHelpModel(statusbar.NewHelpRenderer(c.Theme, kbr)),
		statusBar:   statusbar.NewStatusBarRenderer(c.Theme, 0),
		ViewState:   StateReady,
		viewport:    vp,
		searchInput: si,
	}

	return m
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Handle search mode.
	if m.ViewState == StateSearching {
		return m.handleSearchMode(msg)
	}

	switch msg := msg.(type) {
	case LoadDocumentMsg:
		m.CurrentDocument = msg.Document
		m.SetContent(msg.Document.Body)

		return nil

	case RevisionMsg:
		m.CurrentDocument = msg.Document
		m.AddRevision(msg.Document.Body)

		return nil

	case ExitSearchMsg:
		m.ExitSearch()

		return nil

	case tea.KeyPressMsg:
		cmd := m.keyHandler.HandlePagerKeys(m, msg)
		cmds = append(cmds, cmd)

	// We've received terminal dimensions, either for the first time or
	// after a resize.
	case tea.WindowSizeMsg:
		// Size is handled by SetSize called from ui.go.

	default:
		if m.StatusMessage.Update(msg) {
			return nil
		}
	}

	// Pass mouse events through to yamlviewport for wheel scrolling.
	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

func (m Model) View() string {
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

func (m *Model) SetSize(w, h int) tea.Cmd {
	m.width = w
	m.height = h
	m.Help.SetWidth(w)
	m.statusBar.SetWidth(w)

	// Calculate viewport dimensions.
	viewportHeight := h - statusBarHeight

	// Subtract help height if visible.
	if helpH := m.Help.Height(); helpH > 0 {
		viewportHeight -= (statusBarHeight + helpH)
	}

	m.searchInput.SetWidth(w - ansi.StringWidth(m.searchInput.Prompt))

	m.viewport.SetWidth(w)
	m.viewport.SetHeight(viewportHeight)

	return nil
}

// SetContent sets the YAML content to display.
func (m *Model) SetContent(source *niceyaml.Source) {
	if source == nil || source.IsEmpty() {
		return
	}

	m.viewport.SetTokens(source)
}

// AddRevision adds a new revision for diff tracking.
func (m *Model) AddRevision(source *niceyaml.Source) {
	if source == nil || source.IsEmpty() {
		return
	}

	m.viewport.AddRevision(source)
}

func (m *Model) Unload() {
	slog.Debug("unload pager document")
	if m.Help.Visible() {
		m.ToggleHelp()
	}
	// Clear search state.
	if m.ViewState == StateSearching {
		m.ExitSearch()
	}

	m.showingResult = false
	m.viewport.ClearRevisions()
	m.viewport.ClearSearch()

	m.ViewState = StateReady
	m.viewport.GotoTop()
}

// CurrentDocumentObject returns the kube object of the currently loaded
// document, or nil if no document is loaded.
func (m *Model) CurrentDocumentObject() *kube.Object {
	return m.CurrentDocument.Object
}

// IsShowingResult reports whether the pager is displaying command output
// rather than a regular document.
func (m *Model) IsShowingResult() bool {
	return m.showingResult
}

// SetShowingResult marks the pager as displaying command output.
func (m *Model) SetShowingResult(v bool) {
	m.showingResult = v
}

func (m *Model) ToggleHelp() {
	m.Help.Toggle()
	m.SetSize(m.width, m.height)

	if m.viewport.PastBottom() {
		m.viewport.GotoBottom()
	}
}

func (m Model) statusBarView() string {
	var opts []statusbar.StatusBarOpt
	if opt := m.StatusMessage.Opt(); opt != nil {
		opts = append(opts, opt)
	}

	m.statusBar.Apply(opts...)

	return m.statusBar.RenderWithScroll(m.CurrentDocument.Title, m.viewport.ScrollPercent())
}

func (m Model) helpView() string {
	return m.Help.View(m.width)
}

// searchBarView renders the search input bar.
func (m Model) searchBarView() string {
	return m.searchInput.View()
}

// StartSearch initializes search mode.
func (m *Model) StartSearch() tea.Cmd {
	m.ViewState = StateSearching

	m.searchInput.Reset()
	m.searchInput.CursorEnd()
	m.searchInput.Focus()

	return textinput.Blink
}

// handleSearchMode handles key events when in search mode.
func (m *Model) handleSearchMode(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()

		switch {
		case m.keyBinds.Escape.Match(key):
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
				cmds = append(cmds, m.sendStatusMessage(statusMsg, statusbar.StyleSuccess))
			} else if searchTerm != "" {
				cmds = append(cmds, m.sendStatusMessage("no matches found", statusbar.StyleError))
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

// IsSearching returns whether the pager is in search mode.
func (m *Model) IsSearching() bool {
	return m.ViewState == StateSearching
}

// ExitSearch exits search mode.
func (m *Model) ExitSearch() {
	m.ViewState = StateReady
	m.searchInput.Blur()
	m.searchInput.Reset()
}

// MoveUp moves the viewport up.
func (m *Model) MoveUp() {
	m.viewport.ScrollUp(1)
}

// MoveDown moves the viewport down.
func (m *Model) MoveDown() {
	m.viewport.ScrollDown(1)
}

// PageUp moves the viewport up by one page.
func (m *Model) PageUp() {
	m.viewport.PageUp()
}

// PageDown moves the viewport down by one page.
func (m *Model) PageDown() {
	m.viewport.PageDown()
}

// GoToTop moves to the top of the document.
func (m *Model) GoToTop() {
	m.viewport.GotoTop()
}

// GoToBottom moves to the bottom of the document.
func (m *Model) GoToBottom() {
	m.viewport.GotoBottom()
}

// HalfPageUp moves the viewport up by half a page.
func (m *Model) HalfPageUp() {
	m.viewport.HalfPageUp()
}

// HalfPageDown moves the viewport down by half a page.
func (m *Model) HalfPageDown() {
	m.viewport.HalfPageDown()
}

// NextMatch goes to the next search match.
func (m *Model) NextMatch() tea.Cmd {
	count := m.viewport.SearchCount()
	if count == 0 {
		return m.sendStatusMessage("no matches found", statusbar.StyleError)
	}

	m.viewport.SearchNext()

	idx := m.viewport.SearchIndex()
	statusMsg := fmt.Sprintf("match %d/%d", idx+1, count)

	return m.sendStatusMessage(statusMsg, statusbar.StyleSuccess)
}

// PrevMatch goes to the previous search match.
func (m *Model) PrevMatch() tea.Cmd {
	count := m.viewport.SearchCount()
	if count == 0 {
		return m.sendStatusMessage("no matches found", statusbar.StyleError)
	}

	m.viewport.SearchPrevious()

	idx := m.viewport.SearchIndex()
	statusMsg := fmt.Sprintf("match %d/%d", idx+1, count)

	return m.sendStatusMessage(statusMsg, statusbar.StyleSuccess)
}

// CopyContent copies the current document content to clipboard.
func (m *Model) CopyContent() tea.Cmd {
	content := m.CurrentDocument.Body.Content()

	return tea.Sequence(
		tea.SetClipboard(content),
		func() tea.Msg {
			_ = clipboard.WriteAll(content) //nolint:errcheck // Can be ignored.
			return nil
		},
		m.sendStatusMessage("copied contents", statusbar.StyleSuccess),
	)
}

// sendStatusMessage sets a local status message that auto-clears after
// [statusbar.StatusMessageTimeout].
func (m *Model) sendStatusMessage(msg string, s statusbar.Style) tea.Cmd {
	return m.StatusMessage.Set(msg, s)
}

// ToggleDiffMode cycles between diff modes.
func (m *Model) ToggleDiffMode() {
	m.viewport.ToggleDiffMode()
}

// ToggleViewMode cycles between view modes.
func (m *Model) ToggleViewMode() {
	m.viewport.ToggleViewMode()
}

// ToggleWordWrap toggles word wrapping.
func (m *Model) ToggleWordWrap() {
	m.viewport.ToggleWordWrap()
}
