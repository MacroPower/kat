package menu

import (
	"context"
	"log/slog"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/log"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/configeditor"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/theme"
)

type ChangeConfigMsg struct {
	Context context.Context
	To      configeditor.Result
}

type Model struct {
	cm           *common.CommonModel
	helpRenderer *statusbar.HelpRenderer
	keyHandler   *KeyHandler
	configeditor configeditor.Model
	ShowHelp     bool
}

type Config struct {
	CommonModel *common.CommonModel
	KeyBinds    *KeyBinds
}

// NewModel creates a new menu model with rule-based directory filtering.
func NewModel(c Config) Model {
	kbr := &keys.KeyBindRenderer{}
	ckb := c.CommonModel.KeyBinds
	kb := c.KeyBinds

	kbr.AddColumn(
		*ckb.Up,
		*ckb.Down,
		*ckb.Left,
		*ckb.Right,
	)
	kbr.AddColumn(
		*kb.PageUp,
		*kb.PageDown,
		*kb.Home,
		*kb.End,
	)
	kbr.AddColumn(
		*kb.Select,
		*ckb.Help,
		*ckb.Escape,
		*ckb.Quit,
	)

	m := Model{
		cm:           c.CommonModel,
		keyHandler:   NewKeyHandler(kb, ckb),
		helpRenderer: statusbar.NewHelpRenderer(c.CommonModel.Theme, kbr),
	}
	m.addConfigEditor()

	return m
}

func (m *Model) Init() tea.Cmd {
	m.addConfigEditor()

	return m.configeditor.Init()
}

func (m *Model) addConfigEditor() {
	m.configeditor = configeditor.NewModel(
		m.cm.Cmd,
		theme.HuhTheme(m.cm.Theme),
		m.keyHandler.HuhKeyMap(),
	)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.configeditor.Focused() {
			break
		}

		m, cmd = m.keyHandler.HandleKeys(m, msg)
		cmds = append(cmds, cmd)
	}

	m.configeditor, cmd = m.configeditor.Update(msg)
	cmds = append(cmds, cmd)

	if m.configeditor.IsCompleted() {
		cmds = append(cmds, m.submitResults(context.Background()))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) submitResults(ctx context.Context) tea.Cmd {
	log.WithContext(ctx).DebugContext(ctx, "config editor completed",
		slog.Any("data", m.configeditor.Result()),
	)

	return func() tea.Msg {
		return ChangeConfigMsg{
			To:      m.configeditor.Result(),
			Context: ctx,
		}
	}
}

func (m Model) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.configeditor.View(),
		m.statusBarView(),
		m.helpView(),
	)
}

func (m Model) statusBarView() string {
	return m.cm.GetStatusBar().RenderWithNote("menu", m.cm.Theme.Ellipsis)
}

func (m *Model) SetSize(_, h int) {
	// Calculate help height if needed.
	if m.ShowHelp {
		helpHeight := m.helpRenderer.CalculateHelpHeight()
		m.configeditor.SetHeight(h - helpHeight - 2)
	} else {
		m.configeditor.SetHeight(h - 1)
	}
}

func (m *Model) Unload() {
	// Replace the editor with a new instance.
	m.addConfigEditor()
}

// helpView renders the help content.
func (m Model) helpView() string {
	if !m.ShowHelp {
		return ""
	}

	return m.helpRenderer.Render(m.cm.Width)
}

// ToggleHelp toggles the help display.
func (m *Model) ToggleHelp() {
	m.ShowHelp = !m.ShowHelp
	m.SetSize(m.cm.Width, m.cm.Height)
}
