package pager

import (
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/keys"
	"github.com/MacroPower/kat/pkg/ui/statusbar"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

const statusBarHeight = 1

type (
	ContentRenderedMsg string
)

type ViewState int

const (
	StateReady ViewState = iota
	StateLoadingDocument
	StateShowingError
	StateShowingStatusMessage
)

type PagerModel struct {
	cm              *common.CommonModel
	helpRenderer    *statusbar.HelpRenderer
	glamourRenderer *GlamourRenderer

	// Current document being rendered, sans-glamour rendering. We cache
	// it here so we can re-render it on resize.
	CurrentDocument yamldoc.YAMLDocument

	viewport   viewport.Model
	helpHeight int
	ViewState  ViewState
	ShowHelp   bool
}

func NewPagerModel(cm *common.CommonModel) PagerModel {
	// Init viewport.
	vp := viewport.New(0, 0)
	vp.YPosition = 0

	kb := cm.Config.KeyBinds
	kbr := &keys.KeyBindRenderer{}
	kbr.AddColumn(
		*kb.Common.Up,
		*kb.Common.Down,
		*kb.Pager.PageUp,
		*kb.Pager.PageDown,
		*kb.Pager.HalfPageUp,
		*kb.Pager.HalfPageDown,
	)
	kbr.AddColumn(
		*kb.Pager.Home,
		*kb.Pager.End,
		*kb.Pager.Copy,
		*kb.Common.Reload,
		*kb.Common.Escape,
		*kb.Common.Quit,
	)

	m := PagerModel{
		cm:           cm,
		helpRenderer: statusbar.NewHelpRenderer(kbr),
		ViewState:    StateReady,
		viewport:     vp,
	}

	return m
}

func (m *PagerModel) Init() tea.Cmd {
	return func() tea.Msg {
		gr, err := NewGlamourRenderer(m.cm.Config.GlamourStyle, m.cm.Config.LineNumbersDisabled)
		if err != nil {
			return common.ErrMsg{Err: err}
		}
		m.glamourRenderer = gr

		return nil
	}
}

func (m PagerModel) Update(msg tea.Msg) (PagerModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		kb := m.cm.Config.KeyBinds
		key := msg.String()

		switch {
		case kb.Pager.Home.Match(key):
			m.viewport.GotoTop()

			return m, nil

		case kb.Pager.End.Match(key):
			m.viewport.GotoBottom()

			return m, nil

		case kb.Pager.HalfPageDown.Match(key):
			m.viewport.HalfPageDown()

			return m, nil

		case kb.Pager.HalfPageUp.Match(key):
			m.viewport.HalfPageUp()

			return m, nil

		case kb.Common.Help.Match(key):
			m.toggleHelp()

			return m, nil

		case kb.Pager.Copy.Match(key):
			// Copy using OSC 52.
			termenv.Copy(m.CurrentDocument.Body)
			// Copy using native system clipboard.
			_ = clipboard.WriteAll(m.CurrentDocument.Body) //nolint:errcheck // Can be ignored.
			cmds = append(cmds, m.cm.SendStatusMessage("copied contents", statusbar.StyleSuccess))
		}

	case ContentRenderedMsg:
		m.setContent(string(msg))

	// We've received terminal dimensions, either for the first time or
	// after a resize.
	case tea.WindowSizeMsg:
		return m, m.RenderWithGlamour(m.CurrentDocument.Body)
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m PagerModel) View() string {
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

	// Calculate help height if needed.
	if m.ShowHelp {
		m.helpHeight = m.helpRenderer.CalculateHelpHeight()
		viewportHeight -= (statusBarHeight + m.helpHeight)
	}

	m.viewport.Width = w
	m.viewport.Height = viewportHeight
}

// This is where the magic happens.
func (m PagerModel) RenderWithGlamour(yaml string) tea.Cmd {
	return func() tea.Msg {
		if m.glamourRenderer == nil || m.cm.Config.GlamourDisabled {
			return ContentRenderedMsg(yaml)
		}

		viewMaxWidth := max(0, m.viewport.Width)
		if m.cm.Config.GlamourMaxWidth > 0 {
			viewMaxWidth = min(viewMaxWidth, m.cm.Config.GlamourMaxWidth)
		}

		s, err := m.glamourRenderer.RenderContent(yaml, viewMaxWidth)
		if err != nil {
			log.Debug("error rendering with Glamour", "error", err)

			return common.ErrMsg{Err: err}
		}

		return ContentRenderedMsg(s)
	}
}

func (m *PagerModel) Unload() {
	log.Debug("unload")
	if m.ShowHelp {
		m.toggleHelp()
	}
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
