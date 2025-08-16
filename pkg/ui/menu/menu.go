package menu

import (
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/configeditor"
	"github.com/macropower/kat/pkg/ui/statusbar"
)

type MenuModel struct {
	cm           *common.CommonModel
	helpRenderer *statusbar.HelpRenderer
	configeditor configeditor.Model
	ShowHelp     bool
}

type Config struct {
	CommonModel *common.CommonModel
	KeyBinds    *KeyBinds
}

// NewModel creates a new menu model with rule-based directory filtering.
func NewModel(c Config) MenuModel {
	ce := configeditor.NewModel(&configeditor.Config{
		CommonModel: c.CommonModel,
	})

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

	return MenuModel{
		cm:           c.CommonModel,
		configeditor: ce,
		helpRenderer: statusbar.NewHelpRenderer(c.CommonModel.Theme, kbr),
	}
}

func (m MenuModel) Init() tea.Cmd {
	return m.configeditor.Init()
}

func (m MenuModel) Update(msg tea.Msg) (MenuModel, tea.Cmd) {
	var cmd tea.Cmd

	// Update the configeditor model.
	m.configeditor, cmd = m.configeditor.Update(msg)

	return m, cmd
}

func (m MenuModel) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.configeditor.View(),
		m.statusBarView(),
		m.helpView(),
	)
}

func (m MenuModel) statusBarView() string {
	return m.cm.GetStatusBar().RenderWithNote(".", "...")
}

func (m *MenuModel) SetSize(_, h int) {
	// Calculate help height if needed.
	if m.ShowHelp {
		helpHeight := m.helpRenderer.CalculateHelpHeight()
		m.configeditor.SetHeight(h - helpHeight - 2)
	} else {
		m.configeditor.SetHeight(h - 1)
	}
}

func (m *MenuModel) Unload() {}

// helpView renders the help content.
func (m MenuModel) helpView() string {
	if !m.ShowHelp {
		return ""
	}

	return m.helpRenderer.Render(m.cm.Width)
}

// ToggleHelp toggles the help display.
func (m *MenuModel) ToggleHelp() {
	m.ShowHelp = !m.ShowHelp
	m.SetSize(m.cm.Width, m.cm.Height)
}
