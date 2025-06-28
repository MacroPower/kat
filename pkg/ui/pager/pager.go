package pager

import (
	"log/slog"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/keys"
	"github.com/MacroPower/kat/pkg/ui/common"
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
	cm             *common.CommonModel
	helpRenderer   *statusbar.HelpRenderer
	chromaRenderer *ChromaRenderer
	kb             *KeyBinds

	// Current document being rendered, sans-chroma rendering. We cache
	// it here so we can re-render it on resize.
	CurrentDocument yamldoc.YAMLDocument

	viewport        viewport.Model
	helpHeight      int
	ViewState       ViewState
	ShowHelp        bool
	chromaRendering bool
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
		*kb.Home,
		*kb.End,
		*kb.Copy,
		*ckb.Reload,
		*ckb.Escape,
		*ckb.Quit,
	)

	m := PagerModel{
		cm:              c.CommonModel,
		kb:              c.KeyBinds,
		helpRenderer:    statusbar.NewHelpRenderer(c.CommonModel.Theme, kbr),
		chromaRenderer:  NewChromaRenderer(c.CommonModel.Theme, !c.ShowLineNumbers),
		ViewState:       StateReady,
		viewport:        vp,
		chromaRendering: c.ChromaRendering,
	}

	return m
}

func (m PagerModel) Update(msg tea.Msg) (PagerModel, tea.Cmd) {
	var cmds []tea.Cmd

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

		case m.kb.Copy.Match(key):
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
		return m, m.Render(m.CurrentDocument.Body)
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

func (m *PagerModel) Unload() {
	slog.Debug("unload pager document")
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
