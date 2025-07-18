package pager

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/yamls"
)

const (
	statusBarHeight    = 1
	diffTimeoutSeconds = 3
)

type (
	ContentRenderedMsg string
	ClearDiffTimerMsg  struct{}
)

type ViewState int

const (
	StateReady ViewState = iota
	StateLoadingDocument
	StateShowingError
	StateShowingStatusMessage
	StateSearching
)

type PagerModel struct {
	cm             *common.CommonModel
	helpRenderer   *statusbar.HelpRenderer
	chromaRenderer *yamls.ChromaRenderer
	kb             *KeyBinds
	clearDiffTimer *time.Timer

	// Current document being rendered, sans-chroma rendering. We cache
	// it here so we can re-render it on resize.
	CurrentDocument yamls.Document

	viewport        viewport.Model
	searchInput     textinput.Model
	helpHeight      int
	ViewState       ViewState
	ShowHelp        bool
	chromaRendering bool
	currentMatch    int // Current match index for navigation.
	totalMatches    int // Total number of matches found.
}

type Config struct {
	CommonModel     *common.CommonModel
	KeyBinds        *KeyBinds
	ChromaRendering bool
	ShowLineNumbers bool
}

func NewModel(c Config) PagerModel {
	// Init viewport.
	vp := viewport.New(0, 0)
	vp.YPosition = 0

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
		*ckb.Reload,
		*ckb.Escape,
		*ckb.Error,
		*ckb.Help,
		*ckb.Quit,
		*ckb.Suspend,
	)

	// Add plugin keybinds column if plugins are available.
	profile := c.CommonModel.Cmd.GetCurrentProfile()
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
	si.PromptStyle = c.CommonModel.Theme.FilterStyle.MarginRight(1)
	si.Cursor.Style = c.CommonModel.Theme.CursorStyle.MarginRight(1)
	si.Focus()

	m := PagerModel{
		cm:              c.CommonModel,
		kb:              c.KeyBinds,
		helpRenderer:    statusbar.NewHelpRenderer(c.CommonModel.Theme, kbr),
		chromaRenderer:  yamls.NewChromaRenderer(c.CommonModel.Theme, !c.ShowLineNumbers),
		ViewState:       StateReady,
		viewport:        vp,
		searchInput:     si,
		chromaRendering: c.ChromaRendering,
		currentMatch:    -1,
	}

	return m
}

func (m PagerModel) Update(msg tea.Msg) (PagerModel, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle search mode.
	if m.ViewState == StateSearching {
		return m.handleSearchMode(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		switch {
		case m.kb.Home.Match(key):
			m.viewport.GotoTop()

			return m, nil

		case m.kb.End.Match(key):
			m.viewport.GotoBottom()

			return m, nil

		case m.kb.HalfPageDown.Match(key):
			m.viewport.HalfPageDown()

			return m, nil

		case m.kb.HalfPageUp.Match(key):
			m.viewport.HalfPageUp()

			return m, nil

		case m.cm.KeyBinds.Help.Match(key):
			m.toggleHelp()

			return m, nil

		case m.kb.Search.Match(key):
			cmd := m.startSearch()

			return m, cmd

		case m.kb.NextMatch.Match(key):
			return m.goToNextMatch()

		case m.kb.PrevMatch.Match(key):
			return m.goToPrevMatch()

		case m.kb.Copy.Match(key):
			// Copy using OSC 52.
			termenv.Copy(m.CurrentDocument.Body)
			// Copy using native system clipboard.
			_ = clipboard.WriteAll(m.CurrentDocument.Body) //nolint:errcheck // Can be ignored.
			cmds = append(cmds, m.cm.SendStatusMessage("copied contents", statusbar.StyleSuccess))
		}

	case ContentRenderedMsg:
		m.setContent(string(msg))

		cmds = append(cmds, m.StartClearDiffTimer())

	case ClearDiffTimerMsg:
		if m.chromaRenderer != nil {
			m.chromaRenderer.ClearDiffs()

			cmds = append(cmds, m.Render(m.CurrentDocument.Body))
		}

	// We've received terminal dimensions, either for the first time or
	// after a resize.
	case tea.WindowSizeMsg:
		return m, m.Render(m.CurrentDocument.Body)
	}

	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
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

	// Reserve space for search bar if in search mode.
	if m.ViewState == StateSearching {
		viewportHeight -= statusBarHeight // Search bar takes one line.
	}

	// Calculate help height if needed.
	if m.ShowHelp {
		m.helpHeight = m.helpRenderer.CalculateHelpHeight()
		viewportHeight -= (statusBarHeight + m.helpHeight)
	}

	m.viewport.Width = w
	m.viewport.Height = viewportHeight
}

// This is where the magic happens.
func (m PagerModel) Render(yaml string) tea.Cmd {
	return func() tea.Msg {
		if m.chromaRenderer == nil || !m.chromaRendering {
			return ContentRenderedMsg(yaml)
		}

		s, err := m.chromaRenderer.RenderContent(yaml, max(0, m.viewport.Width))
		if err != nil {
			slog.Debug("error rendering with Chroma",
				slog.Any("error", err),
			)

			return common.ErrMsg{Err: err}
		}

		return ContentRenderedMsg(s)
	}
}

// WaitForClearDiffTimer returns a command that waits for the clear diff timer to expire.
func (m *PagerModel) WaitForClearDiffTimer() tea.Cmd {
	return func() tea.Msg {
		<-m.clearDiffTimer.C
		return ClearDiffTimerMsg{}
	}
}

// StartClearDiffTimer starts a [diffTimeoutSeconds] timer to clear diff highlights.
func (m *PagerModel) StartClearDiffTimer() tea.Cmd {
	if m.clearDiffTimer != nil {
		m.clearDiffTimer.Stop()
	}

	m.clearDiffTimer = time.NewTimer(diffTimeoutSeconds * time.Second)

	return m.WaitForClearDiffTimer()
}

func (m *PagerModel) Unload() {
	slog.Debug("unload pager document")
	if m.ShowHelp {
		m.toggleHelp()
	}
	// Clear search state.
	if m.ViewState == StateSearching {
		m.ExitSearch()
	}
	if m.chromaRenderer != nil {
		m.chromaRenderer.Unload()
	}

	// Stop the clear diff timer if it's running.
	if m.clearDiffTimer != nil {
		m.clearDiffTimer.Stop()

		m.clearDiffTimer = nil
	}

	m.currentMatch = -1
	m.totalMatches = 0
	m.ViewState = StateReady
	m.viewport.SetContent("")

	m.viewport.YOffset = 0
}

func (m *PagerModel) setContent(s string) {
	m.viewport.SetContent(s)
}

func (m *PagerModel) toggleHelp() {
	m.ShowHelp = !m.ShowHelp
	m.SetSize(m.cm.Width, m.cm.Height)

	if m.viewport.PastBottom() {
		m.viewport.GotoBottom()
	}
}

func (m PagerModel) statusBarView() string {
	return m.cm.GetStatusBar().RenderWithScroll(m.CurrentDocument.Title, m.viewport.ScrollPercent())
}

func (m PagerModel) helpView() string {
	var help string
	if m.ShowHelp {
		help = m.helpRenderer.Render(m.cm.Width)
	}

	return help
}

// searchBarView renders the search input bar.
func (m PagerModel) searchBarView() string {
	return m.searchInput.View()
}

// startSearch initializes search mode.
func (m *PagerModel) startSearch() tea.Cmd {
	m.ViewState = StateSearching

	m.searchInput.Reset()
	m.searchInput.CursorEnd()
	m.searchInput.Focus()

	return textinput.Blink
}

// handleSearchMode handles key events when in search mode.
func (m PagerModel) handleSearchMode(msg tea.Msg) (PagerModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		switch {
		case m.cm.KeyBinds.Escape.Match(key):
			// Exit search mode.
			m.ExitSearch()

			return m, nil

		case key == "enter":
			// Apply search and exit search mode.
			var cmd tea.Cmd

			searchTerm := m.searchInput.Value()
			if searchTerm != "" {
				m, cmd = m.applySearch(searchTerm)
				cmds = append(cmds, cmd)
			} else {
				cmds = append(cmds, m.Render(m.CurrentDocument.Body))
			}

			m.ExitSearch()

			return m, tea.Batch(cmds...)
		}
	}

	// Update search input.
	var cmd tea.Cmd

	m.searchInput, cmd = m.searchInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// ExitSearch exits search mode.
func (m *PagerModel) ExitSearch() {
	m.ViewState = StateReady
	m.searchInput.Blur()
	m.searchInput.Reset()
}

// applySearch applies the search term to the content.
func (m PagerModel) applySearch(term string) (PagerModel, tea.Cmd) {
	var cmd tea.Cmd

	if m.chromaRenderer != nil {
		m.chromaRenderer.SetSearchTerm(term)

		// Trigger match finding immediately by calling findMatches on the current content.
		m.chromaRenderer.FindMatchesInContent(m.CurrentDocument.Body)

		// Store the total match count.
		matches := m.chromaRenderer.GetMatches()
		m.totalMatches = len(matches)

		// Reset current match index.
		m.currentMatch = -1

		// Find the first match if available.
		m, cmd = m.goToNextMatch()
	}

	return m, cmd
}

// goToNextMatch navigates to the next search match.
func (m PagerModel) goToNextMatch() (PagerModel, tea.Cmd) {
	if m.chromaRenderer == nil {
		return m, nil
	}

	matches := m.chromaRenderer.GetMatches()
	if len(matches) == 0 {
		return m, m.cm.SendStatusMessage("no matches found", statusbar.StyleError)
	}

	// Move to next match.
	m.currentMatch = (m.currentMatch + 1) % len(matches)
	match := matches[m.currentMatch]

	// Update the renderer with the current selected match.
	m.chromaRenderer.SetCurrentSelectedMatch(m.currentMatch)

	// Calculate line height and scroll to match.
	totalLines := len(strings.Split(m.CurrentDocument.Body, "\n"))
	if totalLines > 0 {
		scrollPercent := float64(match.Line) / float64(totalLines)
		m.viewport.SetYOffset(int(scrollPercent * float64(m.viewport.TotalLineCount())))
	}

	statusMsg := fmt.Sprintf("match %d/%d", m.currentMatch+1, m.totalMatches)

	return m, tea.Batch(
		m.Render(m.CurrentDocument.Body),
		m.cm.SendStatusMessage(statusMsg, statusbar.StyleSuccess),
	)
}

// goToPrevMatch navigates to the previous search match.
func (m PagerModel) goToPrevMatch() (PagerModel, tea.Cmd) {
	if m.chromaRenderer == nil {
		return m, nil
	}

	matches := m.chromaRenderer.GetMatches()
	if len(matches) == 0 {
		return m, m.cm.SendStatusMessage("no matches found", statusbar.StyleError)
	}

	// Move to previous match.
	if m.currentMatch <= 0 {
		m.currentMatch = len(matches) - 1
	} else {
		m.currentMatch--
	}

	match := matches[m.currentMatch]

	// Update the renderer with the current selected match.
	m.chromaRenderer.SetCurrentSelectedMatch(m.currentMatch)

	// Calculate line height and scroll to match.
	totalLines := len(strings.Split(m.CurrentDocument.Body, "\n"))
	if totalLines > 0 {
		scrollPercent := float64(match.Line) / float64(totalLines)
		m.viewport.SetYOffset(int(scrollPercent * float64(m.viewport.TotalLineCount())))
	}

	statusMsg := fmt.Sprintf("match %d/%d", m.currentMatch+1, m.totalMatches)

	return m, tea.Batch(
		m.Render(m.CurrentDocument.Body),
		m.cm.SendStatusMessage(statusMsg, statusbar.StyleSuccess),
	)
}
