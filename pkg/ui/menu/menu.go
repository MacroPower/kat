package menu

import (
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/filepicker"
	"github.com/macropower/kat/pkg/ui/statusbar"
)

type MenuModel struct {
	cm           *common.CommonModel
	helpRenderer *statusbar.HelpRenderer
	keyHandler   *KeyHandler
	filepicker   filepicker.Model
	ShowHelp     bool
}

type Config struct {
	CommonModel *common.CommonModel
	KeyBinds    *KeyBinds
}

// NewModel creates a new menu model with rule-based directory filtering.
func NewModel(c Config) MenuModel {
	fsys, err := c.CommonModel.Cmd.FS()
	if err != nil {
		panic(err)
	}

	fp := filepicker.New(fsys, c.CommonModel.Theme)
	fp.DirAllowed = true
	fp.FileAllowed = false
	fp.ShowSize = true
	fp.ShowPermissions = true

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
		filepicker:   fp,
		keyHandler:   NewKeyHandler(c.KeyBinds, c.CommonModel.KeyBinds),
		helpRenderer: statusbar.NewHelpRenderer(c.CommonModel.Theme, kbr),
	}
}

func (m MenuModel) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m MenuModel) Update(msg tea.Msg) (MenuModel, tea.Cmd) {
	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		m, cmd = m.keyHandler.HandleMenuKeys(m, msg)
		cmds = append(cmds, cmd)
	}

	// Update the filepicker model.
	m.filepicker, cmd = m.filepicker.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m MenuModel) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(
			lipgloss.Bottom,
			m.filepicker.View(),
			lipgloss.NewStyle().
				Height(m.cm.Height).
				Width(2).
				Background(m.cm.Theme.SubtleStyle.GetForeground()).
				Render(""),
			lipgloss.NewStyle().
				Height(m.cm.Height).
				Width(m.cm.Width/2).
				Background(m.cm.Theme.CursorStyle.GetForeground()).
				Render(""),
		),
		m.statusBarView(),
		m.helpView(),
	)
}

func (m MenuModel) statusBarView() string {
	return m.cm.GetStatusBar().RenderWithNote(m.filepicker.CurrentDirectory, "...")
}

func (m *MenuModel) SetSize(w, h int) {
	// Calculate help height if needed.
	if m.ShowHelp {
		helpHeight := m.helpRenderer.CalculateHelpHeight()
		m.filepicker.SetSize(w/2, h-helpHeight-3) // Account for margins.
	} else {
		m.filepicker.SetSize(w/2, h-2)
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

// GoToTop moves the cursor to the top of the file list.
func (m *MenuModel) GoToTop() {
	m.filepicker.GoToTop()
}

// GoToBottom moves the cursor to the bottom of the file list.
func (m *MenuModel) GoToBottom() {
	m.filepicker.GoToLast()
}

// PageUp moves the cursor up by one page in the file list.
func (m *MenuModel) PageUp() {
	m.filepicker.PageUp()
}

// PageDown moves the cursor down by one page in the file list.
func (m *MenuModel) PageDown() {
	m.filepicker.PageDown()
}

// MoveUp moves the cursor up one item in the file list.
func (m *MenuModel) MoveUp() {
	m.filepicker.MoveUp()
}

// MoveDown moves the cursor down one item in the file list.
func (m *MenuModel) MoveDown() {
	m.filepicker.MoveDown()
}

// OpenDirectory opens the currently selected directory.
func (m *MenuModel) OpenDirectory() tea.Cmd {
	var cmd tea.Cmd

	m.filepicker, cmd = m.filepicker.Open()

	return cmd
}

// GoBack navigates to the parent directory.
func (m *MenuModel) GoBack() tea.Cmd {
	var cmd tea.Cmd

	m.filepicker, cmd = m.filepicker.GoBack()

	return cmd
}

// SelectDirectory selects the currently focused directory.
func (m *MenuModel) SelectDirectory() tea.Cmd {
	m.filepicker.Select()
	return nil
}
