package pager

import "github.com/MacroPower/kat/pkg/keys"

type KeyBinds struct {
	Copy *keys.KeyBind

	// Navigation.
	Home         *keys.KeyBind `yaml:"home"`
	End          *keys.KeyBind `yaml:"end"`
	PageUp       *keys.KeyBind `yaml:"pageUp"`
	PageDown     *keys.KeyBind `yaml:"pageDown"`
	HalfPageUp   *keys.KeyBind `yaml:"halfPageUp"`
	HalfPageDown *keys.KeyBind `yaml:"halfPageDown"`

	// Search.
	Search    *keys.KeyBind `yaml:"search"`
	NextMatch *keys.KeyBind `yaml:"nextMatch"`
	PrevMatch *keys.KeyBind `yaml:"prevMatch"`
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
