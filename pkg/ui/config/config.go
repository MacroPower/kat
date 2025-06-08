package config

import (
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/MacroPower/kat/pkg/ui/keys"
)

// Config contains TUI-specific configuration.
type Config struct {
	KeyBinds        *KeyBinds      `json:"keybinds"          kong:"-"                 yaml:"keybinds"`
	MinimumDelay    *time.Duration `json:"minimum-delay"     yaml:"minimum-delay"`
	GlamourStyle    string         `json:"glamour-style"     yaml:"glamour-style"`
	GlamourMaxWidth int            `json:"glamour-max-width" yaml:"glamour-max-width"`
	GlamourDisabled bool           `json:"glamour-disabled"  yaml:"glamour-disabled"`
	ShowLineNumbers bool           `json:"show-line-numbers" yaml:"show-line-numbers"`
}

func (c *Config) EnsureDefaults() {
	if c.KeyBinds == nil {
		c.KeyBinds = NewKeyBinds()
	} else {
		c.KeyBinds.EnsureDefaults()
	}
	if c.MinimumDelay == nil {
		defaultDelay := 500 * time.Millisecond
		c.MinimumDelay = &defaultDelay
	}
}

type KeyBinds struct {
	Common *CommonKeyBinds `json:"common" yaml:"common"`
	Stash  *StashKeyBinds  `json:"stash"  yaml:"stash"`
	Pager  *PagerKeyBinds  `json:"pager"  yaml:"pager"`
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
	Quit    *keys.KeyBind `json:"quit"    yaml:"quit"`
	Suspend *keys.KeyBind `json:"suspend" yaml:"suspend"`
	Reload  *keys.KeyBind `json:"reload"  yaml:"reload"`
	Help    *keys.KeyBind `json:"help"    yaml:"help"`
	Error   *keys.KeyBind `json:"error"   yaml:"error"`
	Escape  *keys.KeyBind `json:"escape"  yaml:"escape"`

	// Navigation.
	Up    *keys.KeyBind `json:"up"    yaml:"up"`
	Down  *keys.KeyBind `json:"down"  yaml:"down"`
	Left  *keys.KeyBind `json:"left"  yaml:"left"`
	Right *keys.KeyBind `json:"right" yaml:"right"`
	Prev  *keys.KeyBind `json:"prev"  yaml:"prev"`
	Next  *keys.KeyBind `json:"next"  yaml:"next"`
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
	Open *keys.KeyBind `json:"open" yaml:"open"`
	Find *keys.KeyBind `json:"find" yaml:"find"`
	Home *keys.KeyBind `json:"home" yaml:"home"`
	End  *keys.KeyBind `json:"end"  yaml:"end"`
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
	Home         *keys.KeyBind `json:"home"         yaml:"home"`
	End          *keys.KeyBind `json:"end"          yaml:"end"`
	PageUp       *keys.KeyBind `json:"pageUp"       yaml:"pageUp"`
	PageDown     *keys.KeyBind `json:"pageDown"     yaml:"pageDown"`
	HalfPageUp   *keys.KeyBind `json:"halfPageUp"   yaml:"halfPageUp"`
	HalfPageDown *keys.KeyBind `json:"halfPageDown" yaml:"halfPageDown"`
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
	KeyBinds:        NewKeyBinds(),
}
