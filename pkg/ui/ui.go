// Package ui provides the main UI for the kat application.
package ui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/command"
	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/list"
	"github.com/MacroPower/kat/pkg/ui/overlay"
	"github.com/MacroPower/kat/pkg/ui/pager"
	"github.com/MacroPower/kat/pkg/ui/statusbar"
	"github.com/MacroPower/kat/pkg/ui/themes"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

// NewProgram returns a new Tea program.
func NewProgram(cfg *Config, cmd common.Commander) *tea.Program {
	log.Debug("starting kat")

	opts := []tea.ProgramOption{tea.WithAltScreen()}
	m := newModel(cfg, cmd)

	return tea.NewProgram(m, opts...)
}

// State is the top-level application State.
type State int

const (
	stateShowList State = iota
	stateShowDocument
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
	}[s]
}

type model struct {
	err          error
	result       string
	cm           *common.CommonModel
	overlay      *overlay.Overlay
	kb           *KeyBinds
	spinner      spinner.Model
	pager        pager.PagerModel
	list         list.ListModel
	state        State
	overlayState OverlayState
}

// unloadDocument unloads a document from the pager. Title that while this
// method alters the model we also need to send along any commands returned.
func (m *model) unloadDocument() {
	m.state = stateShowList
	m.list.ViewState = list.StateReady
	m.pager.Unload()
	m.pager.ShowHelp = false
}

func newModel(cfg *Config, cmd common.Commander) tea.Model {
	theme := cfg.UI.Theme
	profile := cmd.GetCurrentProfile()
	if profile.UI != nil {
		if profile.UI.Theme != "" {
			theme = profile.UI.Theme
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
		Theme:    themes.NewTheme(theme),
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

	m := &model{
		cm:      cm,
		spinner: sp,
		state:   stateShowList,
		pager:   pagerModel,
		list:    listModel,
		overlay: overlay.New(),
		kb:      cfg.KeyBinds,
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
		if m.overlayState == overlayStateError || m.overlayState == overlayStateResult {
			// If we're showing an error, any key exits the error view.
			m.overlayState = overlayStateNone
		} else if m.kb.Common.Error.Match(msg.String()) {
			m.overlayState = overlayStateError
		}

		// Handle global key events that should work anywhere in the app.
		if newModel, cmd, handled := m.handleGlobalKeys(msg); handled {
			return newModel, cmd
		}

		if m.kb.Common.Left.Match(msg.String()) {
			if m.state == stateShowDocument {
				m.unloadDocument()
			}
		}

		// Handle plugin keybinds.
		if profile := m.cm.Cmd.GetCurrentProfile(); profile != nil {
			if pluginName := profile.GetPluginNameByKey(msg.String()); pluginName != "" {
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
		cmds = append(cmds, m.pager.RenderWithChroma(body))

	case pager.ContentRenderedMsg:
		m.state = stateShowDocument

	case command.EventEnd:
		cmds = append(cmds, m.handleResourceUpdate(msg)...)

		m.cm.Loaded = true
		switch msg.Type {
		case command.TypeRun:
			m.overlayState = overlayStateNone
		case command.TypePlugin:
			m.result = msg.Stdout
			m.overlayState = overlayStateResult
		}

		if msg.Error == nil && len(msg.Resources) > 0 {
			statusMsg := fmt.Sprintf("rendered %d manifests", len(msg.Resources))
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
		m.cm.Theme.SubtleStyle.Render("press any key to close"),
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
		m.cm.Theme.SubtleStyle.Render("press any key to close"),
	)
}

func (m *model) loadingView() string {
	return m.spinner.View() + " Rendering..."
}

// handleGlobalKeys handles keys that work across all contexts.
func (m *model) handleGlobalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	key := msg.String()

	switch {
	case m.kb.Common.Quit.Match(key):
		if m.state == stateShowList && m.list.FilterState == list.Filtering {
			// Pass through to filter handler.
			return m, nil, false
		}

		return m, tea.Quit, true

	case m.kb.Common.Suspend.Match(key):
		return m, tea.Suspend, true

	case m.kb.Common.Escape.Match(key):
		if m.state == stateShowDocument || !m.cm.Loaded {
			m.unloadDocument()
		}
		if m.state == stateShowList {
			m.list.ResetFiltering()
		}

		return m, nil, true

	case m.kb.Common.Reload.Match(key):
		return m.handleRefreshKey(msg)
	}

	return m, nil, false
}

// handleRefreshKey handles the refresh key based on current state.
func (m *model) handleRefreshKey(_ tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if m.state == stateShowList && m.list.FilterState == list.Filtering {
		// Pass through to list handler.
		return m, nil, false
	}

	initCmds := m.Init()

	return m, initCmds, true
}

// handleResourceUpdate processes kubernetes resource updates.
func (m *model) handleResourceUpdate(msg command.EventEnd) []tea.Cmd {
	var cmds []tea.Cmd

	if msg.Error != nil {
		cmds = append(cmds, func() tea.Msg {
			return common.ErrMsg{Err: msg.Error}
		})

		return cmds
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
	}

	return cmds
}

// handleWindowResize handles terminal window resize events.
func (m *model) handleWindowResize(msg tea.WindowSizeMsg) {
	m.cm.Width = msg.Width
	m.cm.Height = msg.Height
	m.list.SetSize(msg.Width, msg.Height)
	m.pager.SetSize(msg.Width, msg.Height)
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
func kubeResourceToYAML(res *kube.Resource) *yamldoc.YAMLDocument {
	return &yamldoc.YAMLDocument{
		Object: res.Object,
		Body:   res.YAML,
		Title:  res.Object.GetNamespacedName(),
		Desc:   res.Object.GetGroupKind(),
	}
}

// LogKeyPress logs key presses for debugging (optional, can be enabled via config).
func LogKeyPress(key, context string) {
	if log.GetLevel() == log.DebugLevel {
		log.Debug("key pressed", "key", key, "context", context)
	}
}
