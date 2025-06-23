// Package ui provides the main UI for the kat application.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/config"
	"github.com/MacroPower/kat/pkg/ui/overlay"
	"github.com/MacroPower/kat/pkg/ui/pager"
	"github.com/MacroPower/kat/pkg/ui/stash"
	"github.com/MacroPower/kat/pkg/ui/statusbar"
	"github.com/MacroPower/kat/pkg/ui/themes"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

// NewProgram returns a new Tea program.
func NewProgram(cfg config.Config, cmd common.Commander) *tea.Program {
	log.Debug("starting kat")

	opts := []tea.ProgramOption{tea.WithAltScreen()}
	m := newModel(cfg, cmd)

	return tea.NewProgram(m, opts...)
}

// State is the top-level application State.
type State int

const (
	stateShowStash State = iota
	stateShowDocument
)

type OverlayState int

const (
	overlayStateNone OverlayState = iota
	overlayStateError
	overlayStateLoading
)

func (s State) String() string {
	return map[State]string{
		stateShowStash:    "showing file listing",
		stateShowDocument: "showing document",
	}[s]
}

type model struct {
	err          error
	cm           *common.CommonModel
	overlay      *overlay.Overlay
	spinner      spinner.Model
	pager        pager.PagerModel
	stash        stash.StashModel
	state        State
	overlayState OverlayState
}

// unloadDocument unloads a document from the pager. Title that while this
// method alters the model we also need to send along any commands returned.
func (m *model) unloadDocument() {
	m.state = stateShowStash
	m.stash.ViewState = stash.StateReady
	m.pager.Unload()
	m.pager.ShowHelp = false
}

func newModel(cfg config.Config, cmd common.Commander) tea.Model {
	theme := cfg.UI.Theme
	profile := cmd.GetCurrentProfile()
	if profile.UI.Theme != "" {
		theme = profile.UI.Theme
	}
	if profile.UI.Compact != nil {
		cfg.UI.Compact = profile.UI.Compact
	}
	if profile.UI.WordWrap != nil {
		cfg.UI.WordWrap = profile.UI.WordWrap
	}
	if profile.UI.ChromaRendering != nil {
		cfg.UI.ChromaRendering = profile.UI.ChromaRendering
	}
	if profile.UI.LineNumbers != nil {
		cfg.UI.LineNumbers = profile.UI.LineNumbers
	}
	if profile.UI.MinimumDelay != nil {
		cfg.UI.MinimumDelay = profile.UI.MinimumDelay
	}

	cm := common.CommonModel{
		Config: cfg,
		Cmd:    cmd,
		Theme:  themes.NewTheme(theme),
	}

	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = cm.Theme.GenericTextStyle

	m := &model{
		cm:      &cm,
		spinner: sp,
		state:   stateShowStash,
		pager:   pager.NewPagerModel(&cm),
		stash:   stash.NewStashModel(&cm),
		overlay: overlay.New(),
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
		kb := m.cm.Config.KeyBinds

		if m.overlayState == overlayStateError {
			// If we're showing an error, any key exits the error view.
			m.overlayState = overlayStateNone
		} else if m.cm.Config.KeyBinds.Common.Error.Match(msg.String()) {
			m.overlayState = overlayStateError
		}

		// Handle global key events that should work anywhere in the app.
		if newModel, cmd, handled := m.handleGlobalKeys(msg); handled {
			return newModel, cmd
		}

		if kb.Common.Left.Match(msg.String()) {
			if m.state == stateShowDocument {
				m.unloadDocument()
			}
		}

	// Window size is received when starting up and on every resize.
	case tea.WindowSizeMsg:
		m.handleWindowResize(msg)

	case common.CommandRunStarted:
		m.cm.Loaded = false
		m.cm.ShowStatusMessage = false
		if m.cm.StatusMessageTimer != nil {
			m.cm.StatusMessageTimer.Stop()
		}
		m.overlayState = overlayStateLoading
		cmds = append(cmds, m.spinner.Tick)

	case stash.FetchedYAMLMsg:
		// We've loaded a YAML file's contents for rendering.
		m.pager.CurrentDocument = *msg
		body := msg.Body
		cmds = append(cmds, m.pager.RenderWithGlamour(body))

	case pager.ContentRenderedMsg:
		m.state = stateShowDocument

	case common.CommandRunFinished:
		cmds = append(cmds, m.handleResourceUpdate(msg)...)

		m.cm.Loaded = true
		m.overlayState = overlayStateNone

		if msg.Error == nil {
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
	)

	switch m.state {
	case stateShowDocument:
		s = m.pager.View()
	default:
		s = m.stash.View()
	}

	switch m.overlayState {
	case overlayStateError:
		overlaySizeFraction = 2.0 / 3.0
		s = m.overlay.Place(s, m.errorView(), overlaySizeFraction, errorOverlayStyle)

	case overlayStateLoading:
		overlaySizeFraction = 1.0 / 4.0
		s = m.overlay.Place(s, m.loadingView(), overlaySizeFraction, loadingOverlayStyle)
	}

	return strings.TrimRight(s, " \n")
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
	kb := m.cm.Config.KeyBinds

	switch {
	case kb.Common.Quit.Match(key):
		if m.state == stateShowStash && m.stash.FilterState == stash.Filtering {
			// Pass through to filter handler.
			return m, nil, false
		}

		return m, tea.Quit, true

	case kb.Common.Suspend.Match(key):
		return m, tea.Suspend, true

	case kb.Common.Escape.Match(key):
		if m.state == stateShowDocument || !m.cm.Loaded {
			m.unloadDocument()
		}
		if m.state == stateShowStash {
			m.stash.ResetFiltering()
		}

		return m, nil, true

	case kb.Common.Reload.Match(key):
		return m.handleRefreshKey(msg)
	}

	return m, nil, false
}

// handleRefreshKey handles the refresh key based on current state.
func (m *model) handleRefreshKey(_ tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if m.state == stateShowStash && m.stash.FilterState == stash.Filtering {
		// Pass through to stash handler.
		return m, nil, false
	}

	initCmds := m.Init()

	return m, initCmds, true
}

// handleResourceUpdate processes kubernetes resource updates.
func (m *model) handleResourceUpdate(msg common.CommandRunFinished) []tea.Cmd {
	var cmds []tea.Cmd

	if msg.Error != nil {
		cmds = append(cmds, func() tea.Msg {
			return common.ErrMsg{Err: msg.Error}
		})

		return cmds
	}

	m.stash.YAMLs = nil

	for _, yml := range msg.Resources {
		newYaml := kubeResourceToYAML(yml)
		m.stash.AddYAMLs(newYaml)

		if m.stash.FilterApplied() {
			newYaml.BuildFilterValue()
		}

		if m.state == stateShowDocument && kube.UnstructuredEqual(yml.Object, m.pager.CurrentDocument.Object) {
			cmds = append(cmds, stash.LoadYAML(newYaml))
		}
	}

	if m.stash.FilterApplied() {
		cmds = append(cmds, stash.FilterYAMLs(m.stash))
	}

	return cmds
}

// updateChildModels updates child models based on current state.
func (m *model) updateChildModels(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd

	switch m.state {
	case stateShowStash:
		newStashModel, cmd := m.stash.Update(msg)
		m.stash = newStashModel
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
	m.stash.SetSize(msg.Width, msg.Height)
	m.pager.SetSize(msg.Width, msg.Height)
	m.overlay.SetSize(msg.Width, msg.Height)
}

func (m *model) runCommand() tea.Cmd {
	return func() tea.Msg {
		go m.cm.Cmd.Run()

		return nil
	}
}

// Convert a [kube.Resource] to an internal representation of a YAML document.
func kubeResourceToYAML(res *kube.Resource) *yamldoc.YAMLDocument {
	title := res.Object.GetName()
	if res.Object.GetNamespace() != "" {
		title = fmt.Sprintf("%s/%s", res.Object.GetNamespace(), res.Object.GetName())
	}

	gvk := res.Object.GetObjectKind().GroupVersionKind()
	desc := gvk.Kind
	if gvk.Group != "" {
		desc = fmt.Sprintf("%s/%s", gvk.Group, gvk.Kind)
	}

	return &yamldoc.YAMLDocument{
		Object: res.Object,
		Body:   res.YAML,
		Title:  title,
		Desc:   desc,
	}
}

// LogKeyPress logs key presses for debugging (optional, can be enabled via config).
func LogKeyPress(key, context string) {
	if log.GetLevel() == log.DebugLevel {
		log.Debug("key pressed", "key", key, "context", context)
	}
}
