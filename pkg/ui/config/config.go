package config

import (
	"github.com/hashicorp/go-multierror"

	"github.com/MacroPower/kat/pkg/ui/keys"
)

// Config contains TUI-specific configuration.
type Config struct {
	KeyBinds        *KeyBinds `hidden:""                yaml:"keybinds"`
	GlamourStyle    string    `yaml:"glamour-style"`
	GlamourMaxWidth int       `yaml:"glamour-max-width"`
	GlamourDisabled bool      `yaml:"glamour-disabled"`
	ShowLineNumbers bool      `yaml:"show-line-numbers"`
	EnableMouse     bool      `yaml:"enable-mouse"`
}

type KeyBinds struct {
	Common *CommonKeyBinds `yaml:"common"`
	Stash  *StashKeyBinds  `yaml:"stash"`
	Pager  *PagerKeyBinds  `yaml:"pager"`
}

func NewKeyBinds() *KeyBinds {
	kb := &KeyBinds{
		Common: &CommonKeyBinds{},
		Stash:  &StashKeyBinds{},
		Pager:  &PagerKeyBinds{},
	}
	kb.EnsureDefaults()

	return kb
}

func (kb *KeyBinds) EnsureDefaults() {
	if kb.Common == nil {
		kb.Common = &CommonKeyBinds{}
	}
	if kb.Stash == nil {
		kb.Stash = &StashKeyBinds{}
	}
	if kb.Pager == nil {
		kb.Pager = &PagerKeyBinds{}
	}

	kb.Common.EnsureDefaults()
	kb.Stash.EnsureDefaults()
	kb.Pager.EnsureDefaults()
}

func (kb *KeyBinds) Validate() error {
	var merr *multierror.Error

	merr = multierror.Append(merr, keys.ValidateBinds(
		kb.Common.GetKeyBinds(),
		kb.Stash.GetKeyBinds(),
	))
	merr = multierror.Append(merr, keys.ValidateBinds(
		kb.Common.GetKeyBinds(),
		kb.Pager.GetKeyBinds(),
	))

	return merr.ErrorOrNil()
}

type CommonKeyBinds struct {
	Quit    *keys.KeyBind
	Suspend *keys.KeyBind
	Reload  *keys.KeyBind
	Help    *keys.KeyBind
	Error   *keys.KeyBind
	Escape  *keys.KeyBind

	// Navigation.
	Up    *keys.KeyBind
	Down  *keys.KeyBind
	Left  *keys.KeyBind
	Right *keys.KeyBind
	Prev  *keys.KeyBind
	Next  *keys.KeyBind
}

func (kb *CommonKeyBinds) EnsureDefaults() {
	keys.SetDefaultBind(&kb.Quit, keys.NewBind("quit", keys.New("q")))
	// Always ensure that ctrl+c is bound to quit.
	kb.Quit.AddKey(keys.New("ctrl+c", keys.WithAlias("⌃c"), keys.Hidden()))

	keys.SetDefaultBind(&kb.Suspend,
		keys.NewBind("suspend",
			keys.New("ctrl+z", keys.WithAlias("⌃z"), keys.Hidden()),
		))
	keys.SetDefaultBind(&kb.Reload,
		keys.NewBind("reload",
			keys.New("r"),
		))
	keys.SetDefaultBind(&kb.Escape,
		keys.NewBind("go back",
			keys.New("esc"),
		))
	keys.SetDefaultBind(&kb.Help,
		keys.NewBind("toggle help",
			keys.New("?"),
		))
	keys.SetDefaultBind(&kb.Error,
		keys.NewBind("toggle error",
			keys.New("!"),
		))

	keys.SetDefaultBind(&kb.Up,
		keys.NewBind("move up",
			keys.New("up", keys.WithAlias("↑")),
			keys.New("k"),
		))
	keys.SetDefaultBind(&kb.Down,
		keys.NewBind("move down",
			keys.New("down", keys.WithAlias("↓")),
			keys.New("j"),
		))
	keys.SetDefaultBind(&kb.Left,
		keys.NewBind("move left",
			keys.New("left", keys.WithAlias("←")),
			keys.New("h"),
		))
	keys.SetDefaultBind(&kb.Right,
		keys.NewBind("move right",
			keys.New("right", keys.WithAlias("→")),
			keys.New("l"),
		))
	keys.SetDefaultBind(&kb.Prev,
		keys.NewBind("previous page",
			keys.New("shift+tab", keys.WithAlias("⇧+tab")),
			keys.New("H"),
		))
	keys.SetDefaultBind(&kb.Next,
		keys.NewBind("next page",
			keys.New("tab"),
			keys.New("L"),
		))
}

func (kb *CommonKeyBinds) GetKeyBinds() []keys.KeyBind {
	return []keys.KeyBind{
		*kb.Quit,
		*kb.Suspend,
		*kb.Reload,
		*kb.Escape,
		*kb.Help,
		*kb.Error,
		*kb.Up,
		*kb.Down,
		*kb.Left,
		*kb.Right,
		*kb.Prev,
		*kb.Next,
	}
}

type StashKeyBinds struct {
	Open *keys.KeyBind
	Find *keys.KeyBind
	Home *keys.KeyBind
	End  *keys.KeyBind
}

func (kb *StashKeyBinds) EnsureDefaults() {
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
}

func (kb *StashKeyBinds) GetKeyBinds() []keys.KeyBind {
	return []keys.KeyBind{
		*kb.Open,
		*kb.Find,
	}
}

type PagerKeyBinds struct {
	Copy *keys.KeyBind

	// Navigation.
	Home         *keys.KeyBind
	End          *keys.KeyBind
	PageUp       *keys.KeyBind `yaml:"page-up"`
	PageDown     *keys.KeyBind `yaml:"page-down"`
	HalfPageUp   *keys.KeyBind `yaml:"half-page-up"`
	HalfPageDown *keys.KeyBind `yaml:"half-page-down"`
}

func (kb *PagerKeyBinds) EnsureDefaults() {
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
			keys.New("pgdn"),
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
}

func (kb *PagerKeyBinds) GetKeyBinds() []keys.KeyBind {
	return []keys.KeyBind{
		*kb.Copy,
		*kb.Home,
		*kb.End,
		*kb.PageUp,
		*kb.PageDown,
		*kb.HalfPageUp,
		*kb.HalfPageDown,
	}
}

var DefaultConfig = Config{
	GlamourStyle:    "auto",
	ShowLineNumbers: true,
	EnableMouse:     true,
	KeyBinds:        NewKeyBinds(),
}
