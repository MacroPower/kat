// Package ui provides the main UI for the kat application.
package ui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/list"
	"github.com/macropower/kat/pkg/ui/menu"
	"github.com/macropower/kat/pkg/ui/overlay"
	"github.com/macropower/kat/pkg/ui/pager"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/ui/yamls"
)

// NewProgram returns a new Tea program.
func NewProgram(cfg *Config, cmd common.Commander) *tea.Program {
	slog.Debug("starting kat ui")

	opts := []tea.ProgramOption{tea.WithAltScreen()}
	m := newModel(cfg, cmd)

	return tea.NewProgram(m, opts...)
}

type GotResultMsg command.Output

type ShowResultMsg struct{}

// State is the top-level application State.
type State int

const (
	stateShowList State = iota
	stateShowDocument
	stateShowResult
	stateShowMenu
)

type OverlayState int

const (
	overlayStateNone OverlayState = iota
	overlayStateError
	overlayStateLoading
	overlayStateResult
)

func (s State) String() string {
	return map[State]string{
		stateShowList:     "showing file listing",
		stateShowDocument: "showing document",
		stateShowResult:   "showing result",
		stateShowMenu:     "showing menu",
	}[s]
}

type model struct {
	err          error
	cm           *common.CommonModel
	overlay      *overlay.Overlay
	kb           *KeyBinds
	result       string
	spinner      spinner.Model
	pager        pager.PagerModel
	fullResult   pager.PagerModel
	list         list.ListModel
	menu         menu.MenuModel
	state        State
	overlayState OverlayState
}

// unloadDocument unloads a document from the pager. Title that while this
// method alters the model we also need to send along any commands returned.
func (m *model) unloadDocument() {
	switch m.state {
	case stateShowDocument:
		m.pager.Unload()

		m.pager.ShowHelp = false

	case stateShowResult:
		m.fullResult.Unload()

		m.fullResult.ShowHelp = false

	case stateShowMenu:
		m.menu.Unload()

		m.menu.ShowHelp = false
	}

	m.state = stateShowList
	m.list.ViewState = list.StateReady
}

func newModel(cfg *Config, cmd common.Commander) tea.Model {
	uiTheme := cfg.UI.Theme
	profile := cmd.GetCurrentProfile()
	if profile != nil && profile.UI != nil {
		if profile.UI.Theme != "" {
			uiTheme = profile.UI.Theme
		}
		if profile.UI.Compact != nil {
			cfg.UI.Compact = profile.UI.Compact
		}
		if profile.UI.WordWrap != nil {
			cfg.UI.WordWrap = profile.UI.WordWrap
		}
		if profile.UI.LineNumbers != nil {
			cfg.UI.LineNumbers = profile.UI.LineNumbers
		}
	}

	cm := &common.CommonModel{
		Cmd:      cmd,
		Theme:    theme.New(uiTheme),
		KeyBinds: cfg.KeyBinds.Common,
	}

	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = cm.Theme.GenericTextStyle

	listModel := list.NewModel(list.Config{
		CommonModel: cm,
		KeyBinds:    cfg.KeyBinds.List,
		Compact:     *cfg.UI.Compact,
	})

	pagerModel := pager.NewModel(pager.Config{
		CommonModel:     cm,
		KeyBinds:        cfg.KeyBinds.Pager,
		ChromaRendering: *cfg.UI.ChromaRendering,
		ShowLineNumbers: *cfg.UI.LineNumbers,
	})

	fullResultModel := pager.NewModel(pager.Config{
		CommonModel:     cm,
		KeyBinds:        cfg.KeyBinds.Pager,
		ChromaRendering: *cfg.UI.ChromaRendering,
		ShowLineNumbers: false,
	})

	menuModel := menu.NewModel(menu.Config{
		CommonModel: cm,
		KeyBinds:    cfg.KeyBinds.Menu,
	})

	m := &model{
		cm:         cm,
		spinner:    sp,
		state:      stateShowList,
		pager:      pagerModel,
		list:       listModel,
		menu:       menuModel,
		fullResult: fullResultModel,
		overlay:    overlay.New(cm.Theme),
		kb:         cfg.KeyBinds,
	}

	return m
}

func (m *model) Init() tea.Cmd {
	return m.runCommand()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		if m.matchAction(m.kb.Common.Error, key) {
			if m.state != stateShowResult {
				m.overlayState = overlayStateNone

				cmds = append(cmds, func() tea.Msg {
					return ShowResultMsg{}
				})

				break
			}
			// If we're showing a result, <!> exits the result view.
			m.state = stateShowList
			m.overlayState = overlayStateNone
			m.fullResult.Unload()

			m.fullResult.ShowHelp = false

			break
		}

		if m.overlayState == overlayStateError || m.overlayState == overlayStateResult {
			// If we're showing an error, any key exits the error view.
			m.overlayState = overlayStateNone

			// Don't break, continue to handle the key event.
		}

		// Handle global key events that should work anywhere in the app.
		if newModel, cmd, handled := m.handleGlobalKeys(msg); handled {
			return newModel, cmd
		}

		if m.matchAction(m.kb.Common.Left, key) {
			if m.state == stateShowDocument || m.state == stateShowResult {
				m.unloadDocument()
			}
		}

		// Handle plugin keybinds.
		profile := m.cm.Cmd.GetCurrentProfile()
		if profile != nil && !m.isTextInputFocused() {
			if pluginName := profile.GetPluginNameByKey(key); pluginName != "" {
				cmd := m.runPlugin(pluginName)

				return m, cmd
			}
		}

	// Window size is received when starting up and on every resize.
	case tea.WindowSizeMsg:
		m.handleWindowResize(msg)

	case command.EventStart:
		m.cm.Loaded = false
		m.cm.ShowStatusMessage = false
		if m.cm.StatusMessageTimer != nil {
			m.cm.StatusMessageTimer.Stop()
		}

		m.overlayState = overlayStateLoading
		cmds = append(cmds, m.spinner.Tick)

	case list.FetchedYAMLMsg:
		// We've loaded a YAML file's contents for rendering.
		m.pager.CurrentDocument = *msg
		body := msg.Body
		m.state = stateShowDocument
		cmds = append(cmds, m.pager.Render(body))

	case GotResultMsg:
		m.err = msg.Error
		if msg.Error != nil {
			if msg.Stderr != "" {
				m.err = fmt.Errorf("%w\n\n%s", m.err, msg.Stderr)
			}
			if msg.Stdout != "" {
				m.err = fmt.Errorf("%w\n\n%s", m.err, msg.Stdout)
			}
		}

		m.result = msg.Stdout

		var body string
		if msg.Error != nil {
			body += "# Error\n" + msg.Error.Error() + "\n---\n"
			m.overlayState = overlayStateError
		} else {
			m.overlayState = overlayStateResult
		}

		body += "# Stdout\n" + msg.Stdout + "\n---\n# Stderr\n" + msg.Stderr

		m.fullResult.CurrentDocument = yamls.Document{
			Body:  body,
			Title: "output",
		}

	case ShowResultMsg:
		m.state = stateShowResult
		cmds = append(cmds, m.fullResult.Render(m.fullResult.CurrentDocument.Body))

	case command.EventEnd:
		cmds = append(cmds, m.handleResourceUpdate(msg)...)

		m.cm.Loaded = true
		if msg.Type == command.TypeRun {
			m.overlayState = overlayStateNone
		}

		if msg.Error == nil && len(msg.Resources) > 0 {
			statusMsg := fmt.Sprintf("rendered %d resources", len(msg.Resources))
			cmds = append(cmds, m.cm.SendStatusMessage(statusMsg, statusbar.StyleSuccess))
		}

	case common.StatusMessageTimeoutMsg:
		m.cm.ShowStatusMessage = false

	case common.ErrMsg:
		m.err = msg.Err
		m.overlayState = overlayStateError

	case spinner.TickMsg:
		if !m.cm.Loaded {
			var cmd tea.Cmd

			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Always pass messages to the other models so we can keep them
	// updated, even if the user isn't currently viewing them.
	cmds = append(cmds, m.updateChildModels(msg)...)

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	var (
		s                   string
		overlaySizeFraction float64

		errorOverlayStyle = m.cm.Theme.ErrorOverlayStyle.
					Align(lipgloss.Left).
					Padding(1)

		loadingOverlayStyle = m.cm.Theme.GenericOverlayStyle.
					Align(lipgloss.Center).
					Padding(1)

		resultOverlayStyle = m.cm.Theme.GenericOverlayStyle.
					Align(lipgloss.Left).
					Padding(1)
	)

	switch m.state {
	case stateShowDocument:
		s = m.pager.View()
	case stateShowResult:
		s = m.fullResult.View()
	case stateShowMenu:
		s = m.menu.View()
	default:
		s = m.list.View()
	}

	switch m.overlayState {
	case overlayStateError:
		overlaySizeFraction = 2.0 / 3.0
		s = m.overlay.Place(s, m.errorView(), overlaySizeFraction, errorOverlayStyle)

	case overlayStateLoading:
		overlaySizeFraction = 1.0 / 4.0
		s = m.overlay.Place(s, m.loadingView(), overlaySizeFraction, loadingOverlayStyle)

	case overlayStateResult:
		overlaySizeFraction = 2.0 / 3.0
		s = m.overlay.Place(s, m.resultView(), overlaySizeFraction, resultOverlayStyle)
	}

	return strings.TrimRight(s, " \n")
}

func (m *model) resultView() string {
	resultMsg := "<nil>"
	if m.result != "" {
		resultMsg = m.result
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		m.cm.Theme.ResultTitleStyle.Padding(0, 1).Render("OUTPUT"),
		lipgloss.NewStyle().Padding(1, 0).Render(resultMsg),
	)
}

func (m *model) errorView() string {
	errMsg := "<nil>"
	if m.err != nil {
		errMsg = m.err.Error()
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		m.cm.Theme.ErrorTitleStyle.Padding(0, 1).Render("ERROR"),
		lipgloss.NewStyle().Padding(1, 0).Render(errMsg),
	)
}

func (m *model) loadingView() string {
	return m.spinner.View() + " Rendering..."
}

// handleGlobalKeys handles keys that work across all contexts.
func (m *model) handleGlobalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	key := msg.String()

	// Always allow suspend to work regardless of current focus.
	if m.kb.Common.Suspend.Match(key) {
		return m, tea.Suspend, true
	}

	switch {
	case m.matchAction(m.kb.Common.Quit, key):
		return m, tea.Quit, true

	case m.matchAction(m.kb.Common.Escape, key):
		isShowingDocument := m.state == stateShowDocument && m.pager.ViewState != pager.StateSearching
		isShowingResult := m.state == stateShowResult && m.fullResult.ViewState != pager.StateSearching
		isShowingMenu := m.state == stateShowMenu
		if isShowingDocument || isShowingResult || isShowingMenu || !m.cm.Loaded {
			m.unloadDocument()
		}
		if m.state == stateShowList {
			m.list.ResetFiltering()
		}
		if m.state == stateShowDocument {
			m.pager.ExitSearch()
		}
		if m.state == stateShowResult {
			m.fullResult.ExitSearch()
		}

		return m, nil, true

	case m.matchAction(m.kb.Common.Menu, key):
		m.state = stateShowMenu
		initCmds := m.menu.Init()

		return m, initCmds, true

	case m.matchAction(m.kb.Common.Reload, key):
		initCmds := m.Init()

		return m, initCmds, true
	}

	return m, nil, false
}

func (m *model) matchAction(kb *keys.KeyBind, key string) bool {
	if m.isTextInputFocused() && keys.IsTextInputAction(key) {
		return false
	}

	return kb.Match(key)
}

func (m *model) isTextInputFocused() bool {
	if m.state == stateShowList && m.list.FilterState == list.Filtering {
		// Pass through to list handler.
		return true
	}
	if m.state == stateShowDocument && m.pager.ViewState == pager.StateSearching {
		// Pass through to pager search handler.
		return true
	}
	if m.state == stateShowResult && m.fullResult.ViewState == pager.StateSearching {
		// Pass through to pager search handler.
		return true
	}

	return false
}

// handleResourceUpdate processes kubernetes resource updates.
func (m *model) handleResourceUpdate(msg command.EventEnd) []tea.Cmd {
	var cmds []tea.Cmd

	if msg.Error != nil || msg.Type == command.TypePlugin {
		cmds = append(cmds, func() tea.Msg {
			return GotResultMsg(msg)
		})
	}

	if len(msg.Resources) == 0 {
		return cmds
	}

	m.list.YAMLs = nil

	for _, yml := range msg.Resources {
		newYaml := kubeResourceToYAML(yml)
		m.list.AddYAMLs(newYaml)

		if m.list.FilterApplied() {
			newYaml.BuildFilterValue()
		}

		if m.state == stateShowDocument && kube.ObjectEqual(yml.Object, m.pager.CurrentDocument.Object) {
			cmds = append(cmds, list.LoadYAML(newYaml))
		}
	}

	if m.list.FilterApplied() {
		cmds = append(cmds, list.FilterYAMLs(m.list))
	}

	return cmds
}

// updateChildModels updates child models based on current state.
func (m *model) updateChildModels(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd

	switch m.state {
	case stateShowList:
		newListModel, cmd := m.list.Update(msg)
		m.list = newListModel

		cmds = append(cmds, cmd)

	case stateShowDocument:
		newPagerModel, cmd := m.pager.Update(msg)
		m.pager = newPagerModel

		cmds = append(cmds, cmd)

	case stateShowResult:
		newResultModel, cmd := m.fullResult.Update(msg)
		m.fullResult = newResultModel

		cmds = append(cmds, cmd)

	case stateShowMenu:
		newMenuModel, cmd := m.menu.Update(msg)
		m.menu = newMenuModel

		cmds = append(cmds, cmd)
	}

	return cmds
}

// handleWindowResize handles terminal window resize events.
func (m *model) handleWindowResize(msg tea.WindowSizeMsg) {
	m.cm.Width = msg.Width
	m.cm.Height = msg.Height
	m.list.SetSize(msg.Width, msg.Height)
	m.pager.SetSize(msg.Width, msg.Height)
	m.fullResult.SetSize(msg.Width, msg.Height)
	m.menu.SetSize(msg.Width, msg.Height)
	m.overlay.SetSize(msg.Width, msg.Height)
}

func (m *model) runCommand() tea.Cmd {
	return func() tea.Msg {
		go m.cm.Cmd.Run()

		return nil
	}
}

func (m *model) runPlugin(name string) tea.Cmd {
	return func() tea.Msg {
		slog.Debug("running plugin",
			slog.String("name", name),
		)

		go m.cm.Cmd.RunPlugin(name)

		return nil
	}
}

// Convert a [kube.Resource] to an internal representation of a YAML document.
func kubeResourceToYAML(res *kube.Resource) *yamls.Document {
	return &yamls.Document{
		Object: res.Object,
		Body:   res.YAML,
		Title:  res.Object.GetNamespacedName(),
		Desc:   res.Object.GetGroupKind(),
	}
}

// LogKeyPress logs key presses for debugging (optional, can be enabled via config).
func LogKeyPress(key, context string) {
	slog.Debug("key pressed",
		slog.String("key", key),
		slog.String("context", context),
	)
}
