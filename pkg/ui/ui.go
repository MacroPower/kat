// Package ui provides the main UI for the kat application.
package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	tea "github.com/charmbracelet/bubbletea"
	glamourstyles "github.com/charmbracelet/glamour/styles"
	te "github.com/muesli/termenv"

	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/config"
	"github.com/MacroPower/kat/pkg/ui/overlay"
	"github.com/MacroPower/kat/pkg/ui/pager"
	"github.com/MacroPower/kat/pkg/ui/stash"
	"github.com/MacroPower/kat/pkg/ui/statusbar"
	"github.com/MacroPower/kat/pkg/ui/styles"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

var (
	spinnerStyle = lipgloss.NewStyle().
			Foreground(styles.Gray)

	errorOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(styles.Red).
				Align(lipgloss.Left).
				Padding(1)

	loadingOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(styles.Gray).
				Foreground(styles.Gray).
				Align(lipgloss.Center).
				Padding(1)
)

// NewProgram returns a new Tea program.
func NewProgram(cfg config.Config, cmd common.Commander) *tea.Program {
	log.Debug(
		"Starting kat",
		"glamour",
		!cfg.GlamourDisabled,
	)

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
	resources    chan common.RunOutput
	spinner      spinner.Model
	pager        pager.PagerModel
	stash        stash.StashModel
	state        State
	overlayState OverlayState

	// Prevent concurrent command runs / resource updates.
	resourceMu sync.Mutex
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
	if cfg.GlamourStyle == glamourstyles.AutoStyle {
		if te.HasDarkBackground() {
			cfg.GlamourStyle = glamourstyles.DarkStyle
		} else {
			cfg.GlamourStyle = glamourstyles.LightStyle
		}
	}

	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = spinnerStyle

	cm := common.CommonModel{
		Config: cfg,
		Cmd:    cmd,
	}

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
	return tea.Sequence(
		m.runCommand(),
		m.spinner.Tick,
	)
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
		m.resources = msg.Ch
		m.cm.Loaded = false
		m.cm.ShowStatusMessage = false
		if m.cm.StatusMessageTimer != nil {
			m.cm.StatusMessageTimer.Stop()
		}
		m.overlayState = overlayStateLoading

		cmds = append(cmds, m.getKubeResources())

	case stash.FetchedYAMLMsg:
		// We've loaded a YAML file's contents for rendering.
		m.pager.CurrentDocument = *msg
		body := msg.Body
		cmds = append(cmds, m.pager.RenderWithGlamour(body))

	case pager.ContentRenderedMsg:
		m.state = stateShowDocument

	case common.CommandRunFinished:
		// Use the new handler utility for resource updates.
		cmds = append(cmds, m.handleResourceUpdate(msg)...)

		m.cm.Loaded = true
		if m.overlayState == overlayStateLoading {
			m.overlayState = overlayStateNone
		}

		statusMsg := fmt.Sprintf("rendered %d manifests", len(msg.Out.Resources))
		cmds = append(cmds, m.cm.SendStatusMessage(statusMsg, statusbar.StyleSuccess))

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
		styles.ErrorTitleStyle.Render("ERROR"),
		lipgloss.NewStyle().Padding(1, 0).Render(errMsg),
		styles.SubtleStyle.Render("press any key to close"),
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

	m.stash.YAMLs = nil
	initCmds := m.Init()

	return m, initCmds, true
}

// handleResourceUpdate processes kubernetes resource updates.
func (m *model) handleResourceUpdate(msg common.CommandRunFinished) []tea.Cmd {
	var cmds []tea.Cmd

	if msg.Err != nil {
		cmds = append(cmds, func() tea.Msg {
			return common.ErrMsg{Err: msg.Err}
		})
	}

	for _, yml := range msg.Out.Resources {
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

// COMMANDS.

func (m *model) runCommand() tea.Cmd {
	return func() tea.Msg {
		locked := m.resourceMu.TryLock()
		if !locked {
			log.Debug("command already running, skipping new run")

			return struct{}{}
		}

		log.Debug("runCommand")
		ch := make(chan common.RunOutput)
		go func() {
			defer close(ch)
			defer m.resourceMu.Unlock()

			startTime := time.Now()
			out, err := m.cm.Cmd.Run()
			endTime := time.Now()

			runTime := endTime.Sub(startTime)
			if runTime < *m.cm.Config.MinimumDelay {
				// Add a delay if the command ran faster than MinimumDelay.
				// This prevents the status from flickering in the UI.
				time.Sleep(*m.cm.Config.MinimumDelay - runTime)
			}

			ch <- common.RunOutput{Out: out, Err: err}
		}()

		return common.CommandRunStarted{Ch: ch}
	}
}

func (m *model) getKubeResources() tea.Cmd {
	return func() tea.Msg {
		res := <-m.resources

		// We're done.
		log.Debug("command run finished")

		return common.CommandRunFinished(res)
	}
}

// ETC.

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
