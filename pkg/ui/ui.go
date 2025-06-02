// Package ui provides the main UI for the kat application.
package ui

import (
	"fmt"
	"sync"

	"github.com/charmbracelet/log"

	tea "github.com/charmbracelet/bubbletea"
	glamourstyles "github.com/charmbracelet/glamour/styles"
	te "github.com/muesli/termenv"

	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/ui/common"
	"github.com/MacroPower/kat/pkg/ui/config"
	"github.com/MacroPower/kat/pkg/ui/pager"
	"github.com/MacroPower/kat/pkg/ui/stash"
	"github.com/MacroPower/kat/pkg/ui/yamldoc"
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

func (s State) String() string {
	return map[State]string{
		stateShowStash:    "showing file listing",
		stateShowDocument: "showing document",
	}[s]
}

type model struct {
	pager    pager.PagerModel
	fatalErr error
	common   *common.CommonModel

	resources chan common.RunOutput

	stash stash.StashModel
	state State

	// Prevent concurrent command runs / resource updates.
	resourceMu sync.Mutex
}

// unloadDocument unloads a document from the pager. Title that while this
// method alters the model we also need to send along any commands returned.
func (m *model) unloadDocument() []tea.Cmd {
	m.state = stateShowStash
	m.stash.ViewState = stash.StateReady
	m.pager.Unload()
	m.pager.ShowHelp = false

	var batch []tea.Cmd
	if !m.stash.IsLoading() {
		batch = append(batch, m.stash.Spinner.Tick)
	}

	return batch
}

func newModel(cfg config.Config, cmd common.Commander) tea.Model {
	if cfg.GlamourStyle == glamourstyles.AutoStyle {
		if te.HasDarkBackground() {
			cfg.GlamourStyle = glamourstyles.DarkStyle
		} else {
			cfg.GlamourStyle = glamourstyles.LightStyle
		}
	}

	cm := common.CommonModel{
		Config: cfg,
		Cmd:    cmd,
	}

	m := &model{
		common: &cm,
		state:  stateShowStash,
		pager:  pager.NewPagerModel(&cm),
		stash:  stash.NewStashModel(&cm),
	}

	return m
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.stash.Spinner.Tick}

	cmds = append(cmds, m.runCommand())

	return tea.Batch(cmds...)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If there's been an error, any key exits.
	if m.fatalErr != nil {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		kb := m.common.Config.KeyBinds

		// Handle global key events that should work anywhere in the app.
		if newModel, cmd, handled := m.handleGlobalKeys(msg); handled {
			return newModel, cmd
		}

		if kb.Common.Left.Match(msg.String()) {
			if m.state == stateShowDocument {
				cmds := m.unloadDocument()

				return m, tea.Batch(cmds...)
			}
		}

	// Window size is received when starting up and on every resize.
	case tea.WindowSizeMsg:
		m.handleWindowResize(msg)

	case common.CommandRunStarted:
		m.resources = msg.Ch
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

		// Always pass these messages to the other models so we can keep them
		// updated, even if the user isn't currently viewing them.
		stashModel, cmd := m.stash.Update(msg)
		m.stash = stashModel
		cmds = append(cmds, cmd)

	case stash.FilteredYAMLMsg:
		if m.state == stateShowDocument {
			newStashModel, cmd := m.stash.Update(msg)
			m.stash = newStashModel
			cmds = append(cmds, cmd)
		}
	}

	// Process child models using the new utility.
	cmds = append(cmds, m.updateChildModels(msg)...)

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if m.fatalErr != nil {
		return common.ErrorView(m.fatalErr.Error(), true)
	}

	switch m.state {
	case stateShowDocument:
		return m.pager.View()
	default:
		return m.stash.View()
	}
}

// handleGlobalKeys handles keys that work across all contexts.
func (m *model) handleGlobalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	key := msg.String()
	kb := m.common.Config.KeyBinds

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
		model, cmd := m.handleEscapeKey()

		return model, cmd, true

	case kb.Common.Reload.Match(key):
		return m.handleRefreshKey(msg)
	}

	return m, nil, false
}

// handleEscapeKey handles the escape key based on current state.
func (m *model) handleEscapeKey() (tea.Model, tea.Cmd) {
	if m.state == stateShowDocument || m.stash.ViewState == stash.StateLoadingDocument {
		batch := m.unloadDocument()

		return m, tea.Batch(batch...)
	}

	return m, nil
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
	m.common.Width = msg.Width
	m.common.Height = msg.Height
	m.stash.SetSize(msg.Width, msg.Height)
	m.pager.SetSize(msg.Width, msg.Height)
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

			out, err := m.common.Cmd.Run()
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

	return &yamldoc.YAMLDocument{
		Object: res.Object,
		Body:   res.YAML,
		Title:  title,
		Desc:   fmt.Sprintf("%s/%s", res.Object.GetAPIVersion(), res.Object.GetKind()),
	}
}

// LogKeyPress logs key presses for debugging (optional, can be enabled via config).
func LogKeyPress(key, context string) {
	if log.GetLevel() == log.DebugLevel {
		log.Debug("key pressed", "key", key, "context", context)
	}
}
