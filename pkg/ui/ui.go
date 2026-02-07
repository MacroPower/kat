// Package ui provides the main UI for the kat application.
package ui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
	"go.jacobcolvin.com/niceyaml"
	"go.jacobcolvin.com/niceyaml/style"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/log"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/menu"
	"github.com/macropower/kat/pkg/ui/pager"
	"github.com/macropower/kat/pkg/ui/resourcelist"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/ui/yamls"
)

// mouseThrottleInterval is the minimum time between mouse wheel/motion events.
// Trackpads on macOS fire scroll events at very high rates, causing excessive
// redraws and input lag.
const mouseThrottleInterval = 15 * time.Millisecond

// NewProgram returns a new Tea program.
func NewProgram(cfg *Config, cmd common.Commander, opts ...tea.ProgramOption) *tea.Program {
	slog.Debug("starting kat ui")

	m := newModel(cfg, cmd)

	opts = append(opts, tea.WithFilter(mouseEventFilter))

	return tea.NewProgram(m, opts...)
}

// mouseEventFilter throttles high-frequency mouse wheel and motion events to
// prevent excessive redraws from trackpad scrolling.
func mouseEventFilter(_ tea.Model, msg tea.Msg) tea.Msg {
	switch msg.(type) {
	case tea.MouseWheelMsg, tea.MouseMotionMsg:
		now := time.Now()
		if now.Sub(lastMouseEvent) < mouseThrottleInterval {
			return nil
		}

		lastMouseEvent = now
	}

	return msg
}

var lastMouseEvent time.Time

type GotResultMsg command.Output

type ShowResultMsg struct{}

// State is the top-level application State.
type State int

const (
	stateShowList State = iota
	stateShowDocument
	stateShowMenu
)

type OverlayState int

const (
	overlayStateNone OverlayState = iota
	overlayStateError
	overlayStateLoading
	overlayStateOutput
)

type model struct {
	cmd            common.Commander
	err            error
	theme          *theme.Theme
	kb             *KeyBinds
	resultDocument yamls.Document
	result         string
	list           resourcelist.Model
	menu           menu.Model
	spinner        spinner.Model
	pager          pager.Model
	state          State
	overlayState   OverlayState
	width          int
	height         int
	loaded         bool
}

// unloadDocument unloads a document from the pager. Note that while this
// method alters the model we also need to send along any commands returned.
func (m *model) unloadDocument() tea.Cmd {
	var cmds []tea.Cmd

	switch m.state {
	case stateShowMenu:
		cmds = append(cmds, m.menu.Unload())

		fallthrough

	case stateShowDocument:
		m.pager.Unload()
	}

	m.state = stateShowList

	return tea.Batch(cmds...)
}

func newModel(cfg *Config, cmd common.Commander) tea.Model {
	uiTheme := cfg.UI.Theme
	_, profile := cmd.GetCurrentProfile()
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

	t := theme.New(uiTheme)

	// Create niceyaml Printer with theme styles and gutter config.
	printerOpts := []niceyaml.PrinterOption{
		niceyaml.WithStyles(t.NiceyamlStyles),
	}

	if !*cfg.UI.LineNumbers {
		printerOpts = append(printerOpts, niceyaml.WithGutter(niceyaml.NoGutter()))
	}

	printer := niceyaml.NewPrinter(printerOpts...)
	printer.SetWordWrap(*cfg.UI.WordWrap)

	// Configure search/selected styles in the niceyaml styles.
	ss := t.NiceyamlStyles
	searchBg := t.SelectedStyle.GetForeground()
	ss[style.Search] = ptr(lipgloss.NewStyle().Underline(true).Bold(true).Foreground(searchBg))
	ss[style.SearchSelected] = ptr(t.LogoStyle.Bold(true))

	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = t.GenericTextStyle

	ckb := cfg.KeyBinds.Common

	listModel := resourcelist.NewModel(resourcelist.Config{
		Theme:     t,
		KeyBinds:  cfg.KeyBinds.List,
		CKeyBinds: ckb,
		Cmd:       cmd,
		Compact:   *cfg.UI.Compact,
	})

	pagerModel := pager.NewModel(pager.Config{
		Theme:     t,
		KeyBinds:  cfg.KeyBinds.Pager,
		CKeyBinds: ckb,
		Cmd:       cmd,
		Printer:   printer,
	})

	menuModel, err := menu.NewModel(menu.Config{
		Theme:     t,
		KeyBinds:  cfg.KeyBinds.Menu,
		CKeyBinds: ckb,
		Cmd:       cmd,
	})
	if err != nil {
		slog.Error("creating menu model", slog.Any("error", err))
	}

	m := &model{
		theme:   t,
		cmd:     cmd,
		spinner: sp,
		state:   stateShowList,
		pager:   pagerModel,
		list:    listModel,
		menu:    menuModel,
		kb:      cfg.KeyBinds,
	}

	return m
}

func ptr[T any](v T) *T {
	return &v
}

func (m *model) Init() tea.Cmd {
	return m.runCommand(context.Background())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.matchAction(m.kb.Common.Error, msg) {
			if !m.pager.IsShowingResult() {
				m.overlayState = overlayStateNone

				cmds = append(cmds, common.CmdHandler(ShowResultMsg{}))

				break
			}
			// If we're showing a result, <!> exits the result view.
			cmds = append(cmds, m.unloadDocument())
			m.overlayState = overlayStateNone

			break
		}

		if m.overlayState == overlayStateError || m.overlayState == overlayStateOutput {
			// If we're showing an error, any key exits the error view.
			m.overlayState = overlayStateNone

			// Don't break, continue to handle the key event.
		}

		// Handle global key events that should work anywhere in the app.
		if newModel, cmd, handled := m.handleGlobalKeys(msg); handled {
			return newModel, cmd
		}

		if m.matchAction(m.kb.Common.Left, msg) {
			if m.state == stateShowDocument {
				cmds = append(cmds, m.unloadDocument())
			}
		}

		// Handle plugin keybinds.
		_, profile := m.cmd.GetCurrentProfile()
		if profile != nil && !m.isTextInputFocused() {
			if pluginName := profile.GetPluginNameByKey(msg.String()); pluginName != "" {
				cmd := m.runPlugin(context.Background(), pluginName)

				return m, cmd
			}
		}

	// Window size is received when starting up and on every resize.
	case tea.WindowSizeMsg:
		cmds = append(cmds, m.handleWindowResize(msg))

	case command.EventStart:
		m.loaded = false
		m.list.ClearStatus()

		m.overlayState = overlayStateLoading
		cmds = append(cmds, m.spinner.Tick)

	case resourcelist.FetchedYAMLMsg:
		// We've loaded a YAML file's contents for rendering.
		m.state = stateShowDocument

		cmds = append(cmds, common.CmdHandler(pager.LoadDocumentMsg{Document: *msg}))

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
			m.overlayState = overlayStateOutput
		}

		body += "# Stdout\n" + msg.Stdout + "\n---\n# Stderr\n" + msg.Stderr

		m.resultDocument = yamls.Document{
			Body:  niceyaml.NewSourceFromString(body),
			Title: "output",
		}

	case ShowResultMsg:
		cmds = append(cmds, m.showResultInPager())

	case command.EventEnd:
		cmds = append(cmds, m.handleResourceUpdate(msg)...)

		m.loaded = true
		if msg.Output.Type == command.TypeRun {
			m.overlayState = overlayStateNone
		}

		if msg.Output.Error == nil && len(msg.Output.Resources) > 0 {
			cmds = append(cmds, m.sendStatusMessage(
				fmt.Sprintf("rendered %d resources", len(msg.Output.Resources)),
				statusbar.StyleSuccess,
			))
		}

	case command.EventConfigure:
		initCmds := m.Init()
		cmds = append(cmds, initCmds)

	case command.EventListResources:
		cmds = append(cmds, m.unloadDocument())

	case command.EventOpenResource:
		m.pager.Unload()

		cmds = append(cmds, m.menu.Unload())

		m.state = stateShowDocument

		resource := msg.Resource
		yamlDoc := kubeResourceToYAML(&resource)
		cmds = append(cmds, common.CmdHandler(pager.LoadDocumentMsg{Document: *yamlDoc}))

	case menu.ChangeConfigMsg:
		m.list.SetItems(nil)

		cmds = append(cmds, m.unloadDocument())

		err := m.cmd.ConfigureContext(msg.Context,
			command.WithProfile(msg.To.Profile),
			command.WithPath(msg.To.File),
			command.WithExtraArgs(msg.To.ExtraArgs...),
		)
		if err != nil {
			m.err = err
			m.overlayState = overlayStateError
		}

	case common.ErrMsg:
		m.err = msg.Err
		m.overlayState = overlayStateError

	case spinner.TickMsg:
		if !m.loaded {
			var cmd tea.Cmd

			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Handle status message timeouts.
	m.list.HandleStatusTimeout(msg)

	// Always pass messages to the other models so we can keep them
	// updated, even if the user isn't currently viewing them.
	cmds = append(cmds, m.updateChildModels(msg)...)

	return m, tea.Batch(cmds...)
}

func (m *model) View() tea.View {
	var s string

	switch m.state {
	case stateShowDocument:
		s = m.pager.View()
	case stateShowMenu:
		s = m.menu.View()
	default:
		s = m.list.View()
	}

	var (
		overlayContent string
		overlayStyle   lipgloss.Style
		widthFraction  float64
	)

	switch m.overlayState {
	case overlayStateError:
		overlayContent = m.errorView()
		overlayStyle = m.theme.Error.OverlayStyle.Align(lipgloss.Left).Padding(1)
		widthFraction = 2.0 / 3.0

	case overlayStateLoading:
		overlayContent = m.loadingView()
		overlayStyle = m.theme.GenericOverlayStyle.Align(lipgloss.Center).Padding(1)
		widthFraction = 1.0 / 4.0

	case overlayStateOutput:
		overlayContent = m.resultView()
		overlayStyle = m.theme.GenericOverlayStyle.Align(lipgloss.Left).Padding(1)
		widthFraction = 2.0 / 3.0
	}

	if m.overlayState != overlayStateNone {
		s = m.placeOverlay(s, overlayContent, widthFraction, overlayStyle)
	}

	v := tea.NewView(strings.TrimRight(s, " \n"))
	v.AltScreen = true
	v.BackgroundColor = m.theme.BackgroundColor
	v.WindowTitle = "kat — " + m.cmd.String()

	return v
}

func (m *model) resultView() string {
	resultMsg := "<nil>"
	if m.result != "" {
		resultMsg = m.result
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		m.theme.ResultTitleStyle.Padding(0, 1).Render("OUTPUT"),
		lipgloss.NewStyle().Padding(1, 0).Render(resultMsg),
	)
}

func (m *model) errorView() string {
	errMsg := "<nil>"
	if m.err != nil {
		errMsg = m.err.Error()
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		m.theme.Error.TitleStyle.Padding(0, 1).Render("ERROR"),
		lipgloss.NewStyle().Padding(1, 0).Render(errMsg),
	)
}

func (m *model) loadingView() string {
	return m.spinner.View() + " Rendering..."
}

const (
	overlayMinWidth         = 16
	overlayMinHeightPadding = 8
	overlayWrapChars        = " /-"
)

// placeOverlay composites styled foreground content centered over the
// background using [lipgloss.Compositor] layers. Content that exceeds
// the available height is truncated with a helper message.
func (m *model) placeOverlay(bg, fg string, widthFraction float64, overlayStyle lipgloss.Style) string {
	overlayWidth := clamp(int(float64(m.width)*widthFraction), overlayMinWidth, m.width)

	// Wrap and truncate content to fit.
	wrapped := cellbuf.Wrap(fg, overlayWidth, overlayWrapChars)
	lines := strings.Split(wrapped, "\n")

	maxHeight := m.height - overlayMinHeightPadding
	if maxHeight <= 0 {
		return bg
	}

	if len(lines) > maxHeight {
		lines = lines[:maxHeight]
		maxTextWidth := max(0, overlayWidth-4)
		truncMsg := "output truncated; press <!> to view full output"
		helperText := ansi.Truncate(truncMsg, maxTextWidth, m.theme.Ellipsis)
		lines = append(lines, "", m.theme.SubtleStyle.Render(helperText))
	}

	styledFg := overlayStyle.Width(overlayWidth).Render(strings.Join(lines, "\n"))

	fgW, fgH := lipgloss.Width(styledFg), lipgloss.Height(styledFg)
	bgW, bgH := lipgloss.Width(bg), lipgloss.Height(bg)

	x := clamp(bgW-fgW, 0, bgW) / 2
	y := clamp(bgH-fgH, 0, bgH) / 2

	bgLayer := lipgloss.NewLayer(bg)
	fgLayer := lipgloss.NewLayer(styledFg).X(x).Y(y)

	return lipgloss.NewCompositor(bgLayer, fgLayer).Render()
}

func clamp(v, lower, upper int) int {
	return min(max(v, lower), upper)
}

// handleGlobalKeys handles keys that work across all contexts.
func (m *model) handleGlobalKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	// Always allow suspend to work regardless of current focus.
	if m.kb.Common.Suspend.Match(msg.String()) {
		return m, tea.Suspend, true
	}

	switch {
	case m.matchAction(m.kb.Common.Quit, msg):
		return m, tea.Quit, true

	case m.matchAction(m.kb.Common.Escape, msg):
		isShowingDocument := m.state == stateShowDocument && !m.pager.IsSearching()
		isShowingMenu := m.state == stateShowMenu
		isShowingList := m.state == stateShowList

		var cmds []tea.Cmd
		if isShowingDocument || isShowingMenu || !m.loaded {
			cmds = append(cmds, m.unloadDocument())
		}
		if isShowingList {
			m.list.ResetFiltering()
		}
		if m.state == stateShowDocument {
			cmds = append(cmds, common.CmdHandler(pager.ExitSearchMsg{}))
		}

		return m, tea.Batch(cmds...), true

	case m.matchAction(m.kb.Common.Menu, msg):
		m.state = stateShowMenu
		initCmds := m.menu.Init()

		return m, initCmds, true

	case m.matchAction(m.kb.Common.Reload, msg):
		initCmds := m.Init()

		return m, initCmds, true
	}

	return m, nil, false
}

func (m *model) matchAction(kb *keys.KeyBind, msg tea.KeyPressMsg) bool {
	if m.isTextInputFocused() && msg.Text != "" {
		return false
	}

	return kb.Match(msg.String())
}

func (m *model) isTextInputFocused() bool {
	if m.state == stateShowList && m.list.IsFiltering() {
		// Pass through to list handler.
		return true
	}
	if m.state == stateShowDocument && m.pager.IsSearching() {
		// Pass through to pager search handler.
		return true
	}
	if m.state == stateShowMenu {
		// Pass through to menu.
		return true
	}

	return false
}

// handleResourceUpdate processes kubernetes resource updates.
func (m *model) handleResourceUpdate(msg command.EventEnd) []tea.Cmd {
	var cmds []tea.Cmd

	cmds = append(cmds, m.routeCommandResult(msg.Output)...)

	if len(msg.Output.Resources) == 0 {
		return cmds
	}

	docs := resourcesToDocuments(msg.Output.Resources)

	cmds = append(cmds, m.list.SetItems(docs))
	cmds = append(cmds, m.notifyPagerRevisions(docs)...)

	return cmds
}

// routeCommandResult sends error or plugin output to the result overlay.
func (m *model) routeCommandResult(output command.Output) []tea.Cmd {
	if output.Error != nil || output.Type == command.TypePlugin {
		return []tea.Cmd{common.CmdHandler(GotResultMsg(output))}
	}

	return nil
}

// resourcesToDocuments converts kubernetes resources to YAML documents.
func resourcesToDocuments(resources []*kube.Resource) []*yamls.Document {
	docs := make([]*yamls.Document, 0, len(resources))
	for _, res := range resources {
		docs = append(docs, kubeResourceToYAML(res))
	}

	return docs
}

// notifyPagerRevisions sends revision messages to the pager for any document
// that matches the currently displayed document.
func (m *model) notifyPagerRevisions(docs []*yamls.Document) []tea.Cmd {
	if m.state != stateShowDocument || m.pager.IsShowingResult() {
		return nil
	}

	var cmds []tea.Cmd

	for _, doc := range docs {
		if kube.ObjectEqual(doc.Object, m.pager.CurrentDocumentObject()) {
			cmds = append(cmds, common.CmdHandler(pager.RevisionMsg{Document: *doc}))
		}
	}

	return cmds
}

// updateChildModels updates child models based on current state.
func (m *model) updateChildModels(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd

	switch m.state {
	case stateShowList:
		cmds = append(cmds, m.list.Update(msg))

	case stateShowDocument:
		cmds = append(cmds, m.pager.Update(msg))

	case stateShowMenu:
		cmds = append(cmds, m.menu.Update(msg))
	}

	return cmds
}

// sendStatusMessage sets a status bar message and schedules its auto-clear.
func (m *model) sendStatusMessage(msg string, sty statusbar.Style) tea.Cmd {
	return m.list.SetStatusMessage(msg, sty)
}

// handleWindowResize handles terminal window resize events.
func (m *model) handleWindowResize(msg tea.WindowSizeMsg) tea.Cmd {
	m.width = msg.Width
	m.height = msg.Height

	cmds := make([]tea.Cmd, 0, 3)
	for _, s := range []common.Sizeable{&m.list, &m.pager, &m.menu} {
		cmds = append(cmds, s.SetSize(msg.Width, msg.Height))
	}

	return tea.Batch(cmds...)
}

// showResultInPager swaps the pager content to show the result document.
func (m *model) showResultInPager() tea.Cmd {
	m.state = stateShowDocument
	m.pager.Unload()
	m.pager.SetShowingResult(true)

	return tea.Batch(
		common.CmdHandler(pager.LoadDocumentMsg{Document: m.resultDocument}),
		m.pager.SetSize(m.width, m.height),
	)
}

func (m *model) runCommand(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		go m.cmd.RunContext(ctx)

		return nil
	}
}

func (m *model) runPlugin(ctx context.Context, name string) tea.Cmd {
	return func() tea.Msg {
		log.WithContext(ctx).DebugContext(ctx, "running plugin",
			slog.String("name", name),
		)

		go m.cmd.RunPluginContext(ctx, name)

		return nil
	}
}

// Convert a [kube.Resource] to an internal representation of a YAML document.
func kubeResourceToYAML(res *kube.Resource) *yamls.Document {
	return &yamls.Document{
		Object: res.Object,
		Body:   res.Source,
		Title:  res.Object.GetNamespacedName(),
		Desc:   res.Object.GetGroupKind(),
	}
}
