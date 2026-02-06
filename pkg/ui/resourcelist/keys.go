package resourcelist

import "github.com/macropower/kat/pkg/keys"

// KeyBinds defines keybindings for the list view.
type KeyBinds struct {
	Open     *keys.KeyBind `json:"open,omitempty"`
	Find     *keys.KeyBind `json:"find,omitempty"`
	Home     *keys.KeyBind `json:"home,omitempty"`
	End      *keys.KeyBind `json:"end,omitempty"`
	PageUp   *keys.KeyBind `json:"pageUp,omitempty"`
	PageDown *keys.KeyBind `json:"pageDown,omitempty"`
}

// EnsureDefaults sets default keybindings for any unset bindings.
func (kb *KeyBinds) EnsureDefaults() {
	keys.SetDefaultBind(&kb.Open,
		keys.NewBind("open",
			keys.New("enter", keys.WithAlias("↵")),
		))
	keys.SetDefaultBind(&kb.Find,
		keys.NewBind("find",
			keys.New("/"),
		))
	keys.SetDefaultBind(&kb.Home,
		keys.NewBind("go to start",
			keys.New("home"),
			keys.New("g"),
		))
	keys.SetDefaultBind(&kb.End,
		keys.NewBind("go to end",
			keys.New("end"),
			keys.New("G"),
		))
	keys.SetDefaultBind(&kb.PageUp,
		keys.NewBind("page up",
			keys.New("pgup"),
			keys.New("b"),
			keys.New("u"),
		))
	keys.SetDefaultBind(&kb.PageDown,
		keys.NewBind("page down",
			keys.New("pgdown", keys.WithAlias("pgdn")),
			keys.New("f"),
			keys.New("d"),
		))
}

// GetKeyBinds returns all keybindings for validation.
func (kb *KeyBinds) GetKeyBinds() []keys.KeyBind {
	return []keys.KeyBind{
		*kb.Open,
		*kb.Find,
		*kb.Home,
		*kb.End,
		*kb.PageUp,
		*kb.PageDown,
	}
}
