// Package ui provides the main UI for the kat application.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/log"

	tea "github.com/charmbracelet/bubbletea"
	te "github.com/muesli/termenv"

	"github.com/MacroPower/kat/pkg/kube"
)

const (
	statusMessageTimeout = time.Second * 3 // How long to show status messages.
	ellipsis             = "…"
)

var config Config

type Commander interface {
	Run() (kube.CommandOutput, error)
}

type runOutput struct {
	err error
	out kube.CommandOutput
}

// NewProgram returns a new Tea program.
func NewProgram(cfg Config, cmd Commander) *tea.Program {
	log.Debug(
		"Starting kat",
		"glamour",
		!cfg.GlamourDisabled,
	)

	config = cfg
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if cfg.EnableMouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	m := newModel(cfg, cmd)

	return tea.NewProgram(m, opts...)
}

type errMsg struct{ err error } //nolint:errname // Tea message.

func (e errMsg) Error() string { return e.err.Error() }

type (
	initCommandRunMsg struct {
		ch chan runOutput
	}
)

type (
	commandRunFinished      runOutput
	statusMessageTimeoutMsg applicationContext
)

// applicationContext indicates the area of the application something applies
// to. Occasionally used as an argument to commands and messages.
type applicationContext int

const (
	stashContext applicationContext = iota
	pagerContext
)

// state is the top-level application state.
type state int

const (
	stateShowStash state = iota
	stateShowDocument
)

func (s state) String() string {
	return map[state]string{
		stateShowStash:    "showing file listing",
		stateShowDocument: "showing document",
	}[s]
}

// Common stuff we'll need to access in all models.
type commonModel struct {
	cmd    Commander
	cfg    Config
	width  int
	height int
}

type model struct {
	pager    pagerModel
	fatalErr error
	common   *commonModel

	resources chan runOutput

	stash stashModel
	state state
}

// unloadDocument unloads a document from the pager. Title that while this
// method alters the model we also need to send along any commands returned.
func (m *model) unloadDocument() []tea.Cmd {
	m.state = stateShowStash
	m.stash.viewState = stashStateReady
	m.pager.unload()
	m.pager.showHelp = false

	var batch []tea.Cmd
	if !m.stash.shouldSpin() {
		batch = append(batch, m.stash.spinner.Tick)
	}

	return batch
}

func newModel(cfg Config, cmd Commander) tea.Model {
	initSections()

	if cfg.GlamourStyle == styles.AutoStyle {
		if te.HasDarkBackground() {
			cfg.GlamourStyle = styles.DarkStyle
		} else {
			cfg.GlamourStyle = styles.LightStyle
		}
	}

	common := commonModel{
		cfg: cfg,
		cmd: cmd,
	}

	m := model{
		common: &common,
		state:  stateShowStash,
		pager:  newPagerModel(&common),
		stash:  newStashModel(&common),
	}

	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.stash.spinner.Tick}

	switch m.state {
	case stateShowStash:
		cmds = append(cmds, runCommand(*m.common))
		// case stateShowDocument:
		// 	content, err := os.ReadFile(m.common.cfg.Path)
		// 	if err != nil {
		// 		log.Error("unable to read file", "file", m.common.cfg.Path, "error", err)

		// 		return func() tea.Msg { return errMsg{err} }
		// 	}
		// 	body := string(content)
		// 	cmds = append(cmds, renderWithGlamour(m.pager, body))
	}

	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If there's been an error, any key exits.
	if m.fatalErr != nil {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.state == stateShowDocument || m.stash.viewState == stashStateLoadingDocument {
				batch := m.unloadDocument()

				return m, tea.Batch(batch...)
			}
		case "r":
			var cmd tea.Cmd
			if m.state == stateShowStash {
				// Pass through all keys if we're editing the filter.
				if m.stash.filterState == filtering {
					m.stash, cmd = m.stash.update(msg)

					return m, cmd
				}
				m.stash.yamls = nil

				return m, m.Init()
			}

		case "q":
			var cmd tea.Cmd

			switch m.state {
			case stateShowStash:
				// Pass through all keys if we're editing the filter.
				if m.stash.filterState == filtering {
					m.stash, cmd = m.stash.update(msg)

					return m, cmd
				}
			}

			return m, tea.Quit

		case "left", "h", "delete":
			if m.state == stateShowDocument {
				cmds = append(cmds, m.unloadDocument()...)

				return m, tea.Batch(cmds...)
			}

		case "ctrl+z":
			return m, tea.Suspend

		// Ctrl+C always quits no matter where in the application you are.
		case "ctrl+c":
			return m, tea.Quit
		}

	// Window size is received when starting up and on every resize.
	case tea.WindowSizeMsg:
		m.common.width = msg.Width
		m.common.height = msg.Height
		m.stash.setSize(msg.Width, msg.Height)
		m.pager.setSize(msg.Width, msg.Height)

	case initCommandRunMsg:
		m.resources = msg.ch
		cmds = append(cmds, getKubeResources(m))

	case fetchedYAMLMsg:
		// We've loaded a YAML file's contents for rendering.
		m.pager.currentDocument = *msg
		body := msg.Body
		cmds = append(cmds, renderWithGlamour(m.pager, body))

	case contentRenderedMsg:
		m.state = stateShowDocument

	case commandRunFinished:
		for _, yml := range msg.out.Resources {
			newYaml := kubeResourceToYAML(yml)
			m.stash.addYAMLs(newYaml)
			if m.stash.filterApplied() {
				newYaml.buildFilterValue()
			}
		}
		if m.stash.shouldUpdateFilter() {
			cmds = append(cmds, filterYAMLs(m.stash))
		}
		// Always pass these messages to the stash so we can keep it updated
		// about network activity, even if the user isn't currently viewing
		// the stash.
		stashModel, cmd := m.stash.update(msg)
		m.stash = stashModel
		cmds = append(cmds, cmd)

	// case foundLocalFileMsg:
	// 	newMd := localFileToYAML(m.common.cwd, gitcha.SearchResult(msg))
	// 	m.stash.addYAMLs(newMd)
	// 	if m.stash.filterApplied() {
	// 		newMd.buildFilterValue()
	// 	}
	// 	if m.stash.shouldUpdateFilter() {
	// 		cmds = append(cmds, filterYAMLs(m.stash))
	// 	}
	// 	cmds = append(cmds, findNextLocalFile(m))

	case filteredYAMLMsg:
		if m.state == stateShowDocument {
			newStashModel, cmd := m.stash.update(msg)
			m.stash = newStashModel
			cmds = append(cmds, cmd)
		}
	}

	// Process children.
	switch m.state {
	case stateShowStash:
		newStashModel, cmd := m.stash.update(msg)
		m.stash = newStashModel
		cmds = append(cmds, cmd)

	case stateShowDocument:
		newPagerModel, cmd := m.pager.update(msg)
		m.pager = newPagerModel
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.fatalErr != nil {
		return errorView(m.fatalErr, true)
	}

	switch m.state {
	case stateShowDocument:
		return m.pager.View()
	default:
		return m.stash.view()
	}
}

func errorView(err error, fatal bool) string {
	exitMsg := "press any key to "
	if fatal {
		exitMsg += "exit"
	} else {
		exitMsg += "return"
	}
	s := fmt.Sprintf("%s\n\n%v\n\n%s",
		errorTitleStyle.Render("ERROR"),
		err,
		subtleStyle.Render(exitMsg),
	)

	return "\n" + indent(s, 3)
}

// COMMANDS.

func runCommand(m commonModel) tea.Cmd {
	return func() tea.Msg {
		log.Info("runCommand")

		ch := make(chan runOutput)
		go func() {
			defer close(ch)

			out, err := m.cmd.Run()
			ch <- runOutput{out: out, err: err}
		}()

		return initCommandRunMsg{ch: ch}
	}
}

// func ignorePatterns(m commonModel) []string {
// 	return []string{
// 		m.cfg.Gopath,
// 		"node_modules",
// 		".*",
// 	}
// }

func getKubeResources(m model) tea.Cmd {
	return func() tea.Msg {
		res := <-m.resources

		// We're done.
		log.Debug("command run finished")

		return commandRunFinished(res)
	}
}

// func findNextLocalFile(m model) tea.Cmd {
// 	return func() tea.Msg {
// 		res, ok := <-m.localFileFinder

// 		if ok {
// 			// Okay now find the next one.
// 			return foundLocalFileMsg(res)
// 		}
// 		// We're done.
// 		log.Debug("local file search finished")

// 		return commandRunFinished{}
// 	}
// }

func waitForStatusMessageTimeout(appCtx applicationContext, t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C

		return statusMessageTimeoutMsg(appCtx)
	}
}

// ETC.

// Convert a [kube.Resource] to an internal representation of a YAML document.
func kubeResourceToYAML(res *kube.Resource) *yaml {
	title := res.Object.GetName()
	if res.Object.GetNamespace() != "" {
		title = fmt.Sprintf("%s/%s", res.Object.GetNamespace(), res.Object.GetName())
	}

	return &yaml{
		object: res.Object,
		Body:   res.YAML,
		Title:  title,
		Desc:   fmt.Sprintf("%s/%s", res.Object.GetAPIVersion(), res.Object.GetKind()),
	}
}

// Lightweight version of reflow's indent function.
func indent(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}
	l := strings.Split(s, "\n")
	b := strings.Builder{}
	i := strings.Repeat(" ", n)
	for _, v := range l {
		fmt.Fprintf(&b, "%s%s\n", i, v)
	}

	return b.String()
}
