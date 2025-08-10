package menu

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
)

// KeyBinds defines key bindings for menu view.
type KeyBinds struct {
	// Navigation.
	Home     *keys.KeyBind `json:"home,omitempty"`
	End      *keys.KeyBind `json:"end,omitempty"`
	PageUp   *keys.KeyBind `json:"pageUp,omitempty"`
	PageDown *keys.KeyBind `json:"pageDown,omitempty"`

	// Actions.
	Select *keys.KeyBind `json:"select,omitempty"`
}

// EnsureDefaults sets default key bindings for menu actions.
func (kb *KeyBinds) EnsureDefaults() {
	keys.SetDefaultBind(&kb.Home,
		keys.NewBind("go to top",
			keys.New("home"),
			keys.New("g"),
		))
	keys.SetDefaultBind(&kb.End,
		keys.NewBind("go to bottom",
			keys.New("end"),
			keys.New("G"),
		))
	keys.SetDefaultBind(&kb.PageUp,
		keys.NewBind("page up",
			keys.New("pgup"),
			keys.New("b"),
		))
	keys.SetDefaultBind(&kb.PageDown,
		keys.NewBind("page down",
			keys.New("pgdown", keys.WithAlias("pgdn")),
			keys.New("f"),
		))
	keys.SetDefaultBind(&kb.Select,
		keys.NewBind("select directory",
			keys.New("space"),
		))
}

// GetKeyBinds returns all key bindings.
func (kb *KeyBinds) GetKeyBinds() []keys.KeyBind {
	return []keys.KeyBind{
		*kb.Home,
		*kb.End,
		*kb.PageUp,
		*kb.PageDown,
		*kb.Select,
	}
}

// KeyHandler provides key handling for menu view.
type KeyHandler struct {
	kb  *KeyBinds
	ckb *common.KeyBinds
}

// NewKeyHandler creates a new menu key handler.
func NewKeyHandler(kb *KeyBinds, ckb *common.KeyBinds) *KeyHandler {
	return &KeyHandler{
		kb:  kb,
		ckb: ckb,
	}
}

// HandleMenuKeys handles key events for menu view.
func (h *KeyHandler) HandleMenuKeys(m MenuModel, msg tea.KeyMsg) (MenuModel, tea.Cmd) {
	var cmd tea.Cmd

	key := msg.String()

	switch {
	case h.kb.Home.Match(key):
		m.GoToTop()

	case h.kb.End.Match(key):
		m.GoToBottom()

	case h.kb.PageUp.Match(key):
		m.PageUp()

	case h.kb.PageDown.Match(key):
		m.PageDown()

	case h.ckb.Up.Match(key):
		m.MoveUp()

	case h.ckb.Down.Match(key):
		m.MoveDown()

	case h.ckb.Help.Match(key):
		m.ToggleHelp()

	case h.ckb.Right.Match(key):
		cmd = m.OpenDirectory()

	case h.ckb.Left.Match(key):
		cmd = m.GoBack()

	case h.kb.Select.Match(key):
		cmd = m.SelectDirectory()
	}

	return m, cmd
}
