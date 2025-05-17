// Package ui provides the main UI for the kat application.
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/log"
	"github.com/muesli/gitcha"

	tea "github.com/charmbracelet/bubbletea"
	te "github.com/muesli/termenv"
)

const (
	statusMessageTimeout = time.Second * 3 // How long to show status messages.
	ellipsis             = "…"
)

var config Config

// NewProgram returns a new Tea program.
func NewProgram(cfg Config, content string) *tea.Program {
	log.Debug(
		"Starting kat",
		"glamour",
		cfg.GlamourEnabled,
	)

	config = cfg
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if cfg.EnableMouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	m := newModel(cfg, content)

	return tea.NewProgram(m, opts...)
}

type errMsg struct{ err error } //nolint:errname // Tea message.

func (e errMsg) Error() string { return e.err.Error() }

type (
	initLocalFileSearchMsg struct {
		ch  chan gitcha.SearchResult
		cwd string
	}
)

type (
	foundLocalFileMsg       gitcha.SearchResult
	localFileSearchFinished struct{}
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
	cwd    string
	cfg    Config
	width  int
	height int
}

type model struct {
	pager    pagerModel
	fatalErr error
	common   *commonModel

	// Channel that receives paths to local markdown files
	// via the github.com/muesli/gitcha package.
	localFileFinder chan gitcha.SearchResult

	stash stashModel
	state state
}

// unloadDocument unloads a document from the pager. Note that while this
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

func newModel(cfg Config, content string) tea.Model {
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
	}

	m := model{
		common: &common,
		state:  stateShowStash,
		pager:  newPagerModel(&common),
		stash:  newStashModel(&common),
	}

	path := cfg.Path
	if path == "" && content != "" {
		m.state = stateShowDocument
		m.pager.currentDocument = yaml{Body: content}

		return m
	}

	if path == "" {
		path = "."
	}
	info, err := os.Stat(path)
	if err != nil {
		log.Error("unable to stat file", "file", path, "error", err)
		m.fatalErr = err

		return m
	}
	if info.IsDir() {
		m.state = stateShowStash
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		m.state = stateShowDocument
		m.pager.currentDocument = yaml{
			localPath: path,
			Note:      stripAbsolutePath(path, cwd),
			Modtime:   info.ModTime(),
		}
	}

	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.stash.spinner.Tick}

	switch m.state {
	case stateShowStash:
		cmds = append(cmds, findLocalFiles(*m.common))
	case stateShowDocument:
		content, err := os.ReadFile(m.common.cfg.Path)
		if err != nil {
			log.Error("unable to read file", "file", m.common.cfg.Path, "error", err)

			return func() tea.Msg { return errMsg{err} }
		}
		body := string(content)
		cmds = append(cmds, renderWithGlamour(m.pager, body))
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

	case initLocalFileSearchMsg:
		m.localFileFinder = msg.ch
		m.common.cwd = msg.cwd
		cmds = append(cmds, findNextLocalFile(m))

	case fetchedYAMLMsg:
		// We've loaded a YAML file's contents for rendering.
		m.pager.currentDocument = *msg
		body := msg.Body
		cmds = append(cmds, renderWithGlamour(m.pager, body))

	case contentRenderedMsg:
		m.state = stateShowDocument

	case localFileSearchFinished:
		// Always pass these messages to the stash so we can keep it updated
		// about network activity, even if the user isn't currently viewing
		// the stash.
		stashModel, cmd := m.stash.update(msg)
		m.stash = stashModel

		return m, cmd

	case foundLocalFileMsg:
		newMd := localFileToYAML(m.common.cwd, gitcha.SearchResult(msg))
		m.stash.addYAMLs(newMd)
		if m.stash.filterApplied() {
			newMd.buildFilterValue()
		}
		if m.stash.shouldUpdateFilter() {
			cmds = append(cmds, filterYAMLs(m.stash))
		}
		cmds = append(cmds, findNextLocalFile(m))

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

func findLocalFiles(m commonModel) tea.Cmd {
	return func() tea.Msg {
		log.Info("findLocalFiles")
		var (
			cwd = m.cfg.Path
			err error
		)

		if cwd == "" {
			cwd, err = os.Getwd()
		} else {
			var info os.FileInfo
			info, err = os.Stat(cwd)
			if err == nil && info.IsDir() {
				cwd, err = filepath.Abs(cwd)
			}
		}

		// Note that this is one error check for both cases above.
		if err != nil {
			log.Error("error finding local files", "error", err)

			return errMsg{err}
		}

		log.Debug("local directory is", "cwd", cwd)

		// Switch between FindFiles and FindAllFiles to bypass .gitignore rules.
		var ch chan gitcha.SearchResult
		if m.cfg.ShowAllFiles {
			ch, err = gitcha.FindAllFilesExcept(cwd, yamlGlobs, nil)
		} else {
			ch, err = gitcha.FindFilesExcept(cwd, yamlGlobs, ignorePatterns(m))
		}

		if err != nil {
			log.Error("error finding local files", "error", err)

			return errMsg{err}
		}

		return initLocalFileSearchMsg{ch: ch, cwd: cwd}
	}
}

func ignorePatterns(m commonModel) []string {
	return []string{
		m.cfg.Gopath,
		"node_modules",
		".*",
	}
}

func findNextLocalFile(m model) tea.Cmd {
	return func() tea.Msg {
		res, ok := <-m.localFileFinder

		if ok {
			// Okay now find the next one.
			return foundLocalFileMsg(res)
		}
		// We're done.
		log.Debug("local file search finished")

		return localFileSearchFinished{}
	}
}

func waitForStatusMessageTimeout(appCtx applicationContext, t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C

		return statusMessageTimeoutMsg(appCtx)
	}
}

// ETC.

// Convert a Gitcha result to an internal representation of a YAML
// document. Note that we could be doing things like checking if the file is
// a directory, but we trust that gitcha has already done that.
func localFileToYAML(cwd string, res gitcha.SearchResult) *yaml {
	return &yaml{
		localPath: res.Path,
		Note:      stripAbsolutePath(res.Path, cwd),
		Modtime:   res.Info.ModTime(),
	}
}

func stripAbsolutePath(fullPath, cwd string) string {
	fp, _ := filepath.EvalSymlinks(fullPath) //nolint:errcheck // Can be ignored.
	cp, _ := filepath.EvalSymlinks(cwd)      //nolint:errcheck // Can be ignored.

	return strings.ReplaceAll(fp, cp+string(os.PathSeparator), "")
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
