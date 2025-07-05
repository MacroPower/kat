package pager

import "github.com/macropower/kat/pkg/keys"

type KeyBinds struct {
	Copy *keys.KeyBind `json:"copy,omitempty"`

	// Navigation.
	Home         *keys.KeyBind `json:"home,omitempty"`
	End          *keys.KeyBind `json:"end,omitempty"`
	PageUp       *keys.KeyBind `json:"pageUp,omitempty"`
	PageDown     *keys.KeyBind `json:"pageDown,omitempty"`
	HalfPageUp   *keys.KeyBind `json:"halfPageUp,omitempty"`
	HalfPageDown *keys.KeyBind `json:"halfPageDown,omitempty"`

	// Search.
	Search    *keys.KeyBind `json:"search,omitempty"`
	NextMatch *keys.KeyBind `json:"nextMatch,omitempty"`
	PrevMatch *keys.KeyBind `json:"prevMatch,omitempty"`
}

func (kb *KeyBinds) EnsureDefaults() {
	keys.SetDefaultBind(&kb.Copy,
		keys.NewBind("copy contents",
			keys.New("c"),
		))
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
	keys.SetDefaultBind(&kb.HalfPageUp,
		keys.NewBind("½ page up",
			keys.New("u"),
		))
	keys.SetDefaultBind(&kb.HalfPageDown,
		keys.NewBind("½ page down",
			keys.New("d"),
		))
	keys.SetDefaultBind(&kb.Search,
		keys.NewBind("search content",
			keys.New("/"),
		))
	keys.SetDefaultBind(&kb.NextMatch,
		keys.NewBind("next match",
			keys.New("n"),
		))
	keys.SetDefaultBind(&kb.PrevMatch,
		keys.NewBind("previous match",
			keys.New("N"),
		))
}

func (kb *KeyBinds) GetKeyBinds() []keys.KeyBind {
	return []keys.KeyBind{
		*kb.Copy,
		*kb.Home,
		*kb.End,
		*kb.PageUp,
		*kb.PageDown,
		*kb.HalfPageUp,
		*kb.HalfPageDown,
		*kb.Search,
		*kb.NextMatch,
		*kb.PrevMatch,
	}
}
