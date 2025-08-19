package menu

import (
	"github.com/charmbracelet/huh"

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
		keys.NewBind("select",
			keys.New("enter", keys.WithAlias("↵")),
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

type KeyHandler struct {
	kb  *KeyBinds
	ckb *common.KeyBinds
}

func NewKeyHandler(kb *KeyBinds, ckb *common.KeyBinds) *KeyHandler {
	return &KeyHandler{
		kb:  kb,
		ckb: ckb,
	}
}

func (h *KeyHandler) HandleKeys(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	key := msg.String()

	switch {
	case h.ckb.Help.Match(key):
		m.ToggleHelp()
	}

	return m, cmd
}

func (h *KeyHandler) HuhKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()

	km.Input.Prev = h.ckb.Prev.BubbleKey()
	km.Input.Next = h.ckb.Next.BubbleKey()
	km.Input.Submit = h.kb.Select.BubbleKey()

	km.FilePicker.GotoTop = h.kb.Home.BubbleKey()
	km.FilePicker.GotoBottom = h.kb.End.BubbleKey()
	km.FilePicker.PageUp = h.kb.PageUp.BubbleKey()
	km.FilePicker.PageDown = h.kb.PageDown.BubbleKey()
	km.FilePicker.Back = h.ckb.Left.BubbleKey()
	km.FilePicker.Up = h.ckb.Up.BubbleKey()
	km.FilePicker.Down = h.ckb.Down.BubbleKey()
	km.FilePicker.Close = h.ckb.Escape.BubbleKey()
	km.FilePicker.Prev = h.ckb.Prev.BubbleKey()
	km.FilePicker.Next = h.ckb.Next.BubbleKey()
	km.FilePicker.Select = h.kb.Select.BubbleKey()
	km.FilePicker.Submit = h.kb.Select.BubbleKey()
	// Note: `km.FilePicker.Open = h.ckb.Right.BubbleKey()` unset.

	km.Text.Prev = h.ckb.Prev.BubbleKey()
	km.Text.Next = h.ckb.Next.BubbleKey()
	km.Text.Submit = h.kb.Select.BubbleKey()

	km.Note.Prev = h.ckb.Prev.BubbleKey()
	km.Note.Next = h.ckb.Next.BubbleKey()
	km.Note.Submit = h.kb.Select.BubbleKey()

	return km
}
