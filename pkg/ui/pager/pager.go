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
	"github.com/MacroPower/kat/pkg/ui/stash"
	"github.com/MacroPower/kat/pkg/ui/statusbar"
	"github.com/MacroPower/kat/pkg/ui/view"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

type (
	ContentRenderedMsg string
	reloadMsg          struct{}
)

type pagerState int

const (
	pagerStateBrowse pagerState = iota
	pagerStateStatusMessage
)

type PagerModel struct {
	common             *common.CommonModel
	statusMessageTimer *time.Timer
	helpRenderer       *statusbar.HelpRenderer

	// Current document being rendered, sans-glamour rendering. We cache
	// it here so we can re-render it on resize.
	CurrentDocument yamldoc.YAMLDocument

	statusMessage string
	viewport      viewport.Model
	helpHeight    int
	state         pagerState
	ShowHelp      bool
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
		state:        pagerStateBrowse,
		viewport:     vp,
	}

	return m
}

func (m *PagerModel) SetSize(w, h int) {
	// Use the new PagerLayoutCalculator utility.
	calc := NewPagerLayoutCalculator(w, h)

	// Calculate help height if needed.
	if m.ShowHelp && m.helpHeight == 0 {
		m.helpHeight = m.helpRenderer.CalculateHelpHeight()
	}

	// Calculate viewport dimensions.
	viewportHeight := calc.CalculateViewportHeight(m.ShowHelp, m.helpHeight)

	m.viewport.Width = w
	m.viewport.Height = viewportHeight
}

func (m *PagerModel) setContent(s string) {
	m.viewport.SetContent(s)
}

func (m *PagerModel) toggleHelp() {
	m.ShowHelp = !m.ShowHelp
	m.SetSize(m.common.Width, m.common.Height)

	// Use the layout calculator to validate scroll position.
	calc := NewPagerLayoutCalculator(m.common.Width, m.common.Height)
	calc.ValidateScrollPosition(m.viewport.PastBottom(), m.viewport.GotoBottom)
}

type pagerStatusMessage struct {
	message string
	isError bool
}

// Perform stuff that needs to happen after a successful markdown stash. Note
// that the the returned command should be sent back the through the pager
// update function.
func (m *PagerModel) showStatusMessage(msg pagerStatusMessage) tea.Cmd {
	// Show a success message to the user.
	m.state = pagerStateStatusMessage
	m.statusMessage = msg.message
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}
	m.statusMessageTimer = time.NewTimer(common.StatusMessageTimeout)

	return waitForStatusMessageTimeout(common.PagerContext, m.statusMessageTimer)
}

func (m *PagerModel) Unload() {
	log.Debug("unload")
	if m.ShowHelp {
		m.toggleHelp()
	}
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}
	m.state = pagerStateBrowse
	m.viewport.SetContent("")
	m.viewport.YOffset = 0
}

func (m PagerModel) Update(msg tea.Msg) (PagerModel, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		kb := m.common.Config.KeyBinds
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

		case kb.Common.Escape.Match(key):
			if m.state != pagerStateBrowse {
				m.state = pagerStateBrowse

				return m, nil
			}

		case kb.Pager.Copy.Match(key):
			// Copy using OSC 52.
			termenv.Copy(m.CurrentDocument.Body)
			// Copy using native system clipboard.
			_ = clipboard.WriteAll(m.CurrentDocument.Body) //nolint:errcheck // Can be ignored.
			cmds = append(cmds, m.showStatusMessage(pagerStatusMessage{"Copied contents", false}))
		}

	// App has rendered the content.
	case ContentRenderedMsg:
		log.Debug("content rendered", "state", m.state)

		m.setContent(string(msg))

	// The file was changed and we're reloading it.
	case reloadMsg:
		return m, stash.LoadYAML(&m.CurrentDocument)

	// We've received terminal dimensions, either for the first time or
	// after a resize.
	case tea.WindowSizeMsg:
		return m, m.RenderWithGlamour(m.CurrentDocument.Body)

	case common.StatusMessageTimeoutMsg:
		m.state = pagerStateBrowse
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m PagerModel) View() string {
	return view.NewViewBuilder().
		AddSection(m.viewport.View()).
		AddSection(m.statusBarView()).
		AddSection(m.helpView()).
		Build()
}

func (m PagerModel) statusBarView() string {
	renderer := statusbar.NewStatusBarRenderer(m.common.Width)

	statusMessage := ""
	if m.state == pagerStateStatusMessage {
		statusMessage = m.statusMessage
	}

	return renderer.RenderWithScroll(
		m.CurrentDocument.Title,
		statusMessage,
		m.viewport.ScrollPercent(),
	)
}

func (m PagerModel) helpView() string {
	var help string
	if m.ShowHelp {
		help = m.helpRenderer.Render(m.common.Width)
	}

	return help
}

// This is where the magic happens.
func (m PagerModel) RenderWithGlamour(yaml string) tea.Cmd {
	return func() tea.Msg {
		s, err := NewGlamourRenderer(m).RenderContent(yaml)
		if err != nil {
			log.Error("error rendering with Glamour", "error", err)

			return common.ErrMsg{Err: err}
		}

		return ContentRenderedMsg(s)
	}
}

func waitForStatusMessageTimeout(appCtx common.ApplicationContext, t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C

		return common.StatusMessageTimeoutMsg(appCtx)
	}
}
