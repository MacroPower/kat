package pager

import (
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/keys"
	"github.com/MacroPower/kat/pkg/ui/statusbar"
	"github.com/MacroPower/kat/pkg/ui/view"
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
	common       *common.CommonModel
	helpRenderer *statusbar.HelpRenderer

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
		common:       cm,
		helpRenderer: statusbar.NewHelpRenderer(kbr),
		ViewState:    StateReady,
		viewport:     vp,
	}

	return m
}

func (m PagerModel) Update(msg tea.Msg) (PagerModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		kb := m.common.Config.KeyBinds
		key := msg.String()

		if m.ViewState == StateShowingError {
			// If we're showing an error, any key exits the error view.
			m.ViewState = StateReady

			return m, nil
		}

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

		case kb.Common.Escape.Match(key):
			if m.ViewState != StateReady {
				m.ViewState = StateReady

				return m, nil
			}

		case kb.Common.Error.Match(key):
			if m.ViewState != StateShowingError {
				m.ViewState = StateShowingError

				return m, nil
			}

		case kb.Pager.Copy.Match(key):
			// Copy using OSC 52.
			termenv.Copy(m.CurrentDocument.Body)
			// Copy using native system clipboard.
			_ = clipboard.WriteAll(m.CurrentDocument.Body) //nolint:errcheck // Can be ignored.
			cmds = append(cmds, m.showStatusMessage(common.StatusMessage{Message: "Copied contents", IsError: false}))
		}

	case ContentRenderedMsg:
		m.setContent(string(msg))
		cmds = append(cmds, m.showStatusMessage(common.StatusMessage{Message: "Loaded YAML", IsError: false}))

	case common.ErrMsg:
		cmds = append(cmds, m.showStatusMessage(common.StatusMessage{Message: msg.Err.Error(), IsError: true}))

	// We've received terminal dimensions, either for the first time or
	// after a resize.
	case tea.WindowSizeMsg:
		return m, m.RenderWithGlamour(m.CurrentDocument.Body)

	case common.StatusMessageTimeoutMsg:
		if m.ViewState == StateShowingStatusMessage {
			m.ViewState = StateReady
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m PagerModel) View() string {
	switch m.ViewState {
	case StateShowingError:
		return common.ErrorView(m.common.StatusMessage.Message, false)

	case StateLoadingDocument, StateReady, StateShowingStatusMessage:
		return view.NewViewBuilder().
			AddSection(m.viewport.View()).
			AddSection(m.statusBarView()).
			AddSection(m.helpView()).
			Build()
	}

	return common.ErrorView("unknown application state", true)
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
		s, err := NewGlamourRenderer(m).RenderContent(yaml)
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
	if m.common.StatusMessageTimer != nil {
		m.common.StatusMessageTimer.Stop()
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
	m.SetSize(m.common.Width, m.common.Height)

	if m.viewport.PastBottom() {
		m.viewport.GotoBottom()
	}
}

// Perform stuff that needs to happen after a successful markdown stash. Note
// that the the returned command should be sent back the through the pager
// update function.
func (m *PagerModel) showStatusMessage(msg common.StatusMessage) tea.Cmd {
	// Show a success message to the user.
	m.ViewState = StateShowingStatusMessage
	m.common.StatusMessage = msg
	if m.common.StatusMessageTimer != nil {
		m.common.StatusMessageTimer.Stop()
	}
	m.common.StatusMessageTimer = time.NewTimer(common.StatusMessageTimeout)

	return common.WaitForStatusMessageTimeout(common.PagerContext, m.common.StatusMessageTimer)
}

func (m PagerModel) statusBarView() string {
	return m.common.GetStatusBar(m.ViewState == StateShowingStatusMessage).
		RenderWithScroll(m.CurrentDocument.Title, m.viewport.ScrollPercent())
}

func (m PagerModel) helpView() string {
	var help string
	if m.ShowHelp {
		help = m.helpRenderer.Render(m.common.Width)
	}

	return help
}
