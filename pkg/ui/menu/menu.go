package menu

import (
	"context"
	"fmt"
	"log/slog"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"

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
	cmd          common.Commander
	theme        *theme.Theme
	keyHandler   *KeyHandler
	configeditor configeditor.Model
	Help         statusbar.HelpModel
	width        int
	height       int
}

type Config struct {
	Theme     *theme.Theme
	KeyBinds  *KeyBinds
	CKeyBinds *common.KeyBinds
	Cmd       common.Commander
}

// NewModel creates a new menu model with rule-based directory filtering.
func NewModel(c Config) (Model, error) {
	kbr := &keys.KeyBindRenderer{}
	ckb := c.CKeyBinds
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
		theme:      c.Theme,
		cmd:        c.Cmd,
		keyHandler: NewKeyHandler(kb, ckb),
		Help:       statusbar.NewHelpModel(statusbar.NewHelpRenderer(c.Theme, kbr)),
	}

	if err := m.addConfigEditor(); err != nil {
		return Model{}, fmt.Errorf("initializing menu: %w", err)
	}

	return m, nil
}

func (m *Model) Init() tea.Cmd {
	if err := m.addConfigEditor(); err != nil {
		return func() tea.Msg {
			return common.ErrMsg{Err: err}
		}
	}

	return m.configeditor.Init()
}

func (m *Model) addConfigEditor() error {
	var err error

	m.configeditor, err = configeditor.NewModel(
		m.cmd,
		theme.HuhTheme(m.theme),
		m.keyHandler.HuhKeyMap(),
	)
	if err != nil {
		return fmt.Errorf("creating config editor: %w", err)
	}

	return nil
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.configeditor.Focused() {
			break
		}

		cmd := m.keyHandler.HandleKeys(m, msg)
		cmds = append(cmds, cmd)
	}

	var cmd tea.Cmd

	m.configeditor, cmd = m.configeditor.Update(msg)
	cmds = append(cmds, cmd)

	if m.configeditor.IsCompleted() {
		cmds = append(cmds, m.submitResults(context.Background()))
	}

	return tea.Batch(cmds...)
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
	return statusbar.NewStatusBarRenderer(m.theme, m.width).RenderWithNote("menu", m.theme.Ellipsis)
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h

	if helpH := m.Help.Height(); helpH > 0 {
		m.configeditor.SetHeight(h - helpH - 2)
	} else {
		m.configeditor.SetHeight(h - 1)
	}
}

func (m *Model) Unload() tea.Cmd {
	// Replace the editor with a new instance.
	if err := m.addConfigEditor(); err != nil {
		return func() tea.Msg {
			return common.ErrMsg{Err: err}
		}
	}

	return nil
}

// helpView renders the help content.
func (m Model) helpView() string {
	return m.Help.View(m.width)
}

// ToggleHelp toggles the help display.
func (m *Model) ToggleHelp() {
	m.Help.Toggle()
	m.SetSize(m.width, m.height)
}
