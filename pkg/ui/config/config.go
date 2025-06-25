package config

import (
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/hashicorp/go-multierror"

	"github.com/MacroPower/kat/pkg/ui/keys"
)

var DefaultConfig = NewConfig()

// Config contains TUI-specific configuration.
type Config struct {
	KeyBinds *KeyBinds              `yaml:"keybinds"`
	Themes   map[string]ThemeConfig `validate:"dive"     yaml:"themes,omitempty"`
	UI       *UIConfig              `yaml:"ui,omitempty"`
}

func NewConfig() *Config {
	c := &Config{}
	c.EnsureDefaults()

	return c
}

type UIConfig struct {
	MinimumDelay    *time.Duration `yaml:"minimumDelay"`
	Compact         *bool          `yaml:"compact"`
	WordWrap        *bool          `yaml:"wordWrap"`
	ChromaRendering *bool          `yaml:"chromaRendering"`
	LineNumbers     *bool          `yaml:"lineNumbers"`
	Theme           string         `yaml:"theme"`
}

func (c *UIConfig) EnsureDefaults() {
	if c.MinimumDelay == nil {
		defaultDelay := 200 * time.Millisecond
		c.MinimumDelay = &defaultDelay
	}
}

type ThemeConfig struct {
	Styles chroma.StyleEntries `yaml:"styles,omitempty"`
}

func (c *Config) EnsureDefaults() {
	if c.KeyBinds == nil {
		c.KeyBinds = NewKeyBinds()
	} else {
		c.KeyBinds.EnsureDefaults()
	}

	if c.Themes == nil {
		c.Themes = make(map[string]ThemeConfig)
	}

	if c.UI == nil {
		c.UI = &UIConfig{}
	}
	c.UI.EnsureDefaults()

	// Set defaults for UIConfig in this Config context only.
	setDefaultBool(&c.UI.Compact, false)
	setDefaultBool(&c.UI.WordWrap, true)
	setDefaultBool(&c.UI.ChromaRendering, true)
	setDefaultBool(&c.UI.LineNumbers, true)
}

func setDefaultBool(b **bool, value bool) {
	if *b == nil {
		*b = &value
	}
}

type KeyBinds struct {
	Common *CommonKeyBinds `yaml:"common"`
	List   *ListKeyBinds   `yaml:"list"`
	Pager  *PagerKeyBinds  `yaml:"pager"`
}

func NewKeyBinds() *KeyBinds {
	kb := &KeyBinds{
		Common: &CommonKeyBinds{},
		List:   &ListKeyBinds{},
		Pager:  &PagerKeyBinds{},
	}
	kb.EnsureDefaults()

	return kb
}

func (kb *KeyBinds) EnsureDefaults() {
	if kb.Common == nil {
		kb.Common = &CommonKeyBinds{}
	}
	if kb.List == nil {
		kb.List = &ListKeyBinds{}
	}
	if kb.Pager == nil {
		kb.Pager = &PagerKeyBinds{}
	}

	kb.Common.EnsureDefaults()
	kb.List.EnsureDefaults()
	kb.Pager.EnsureDefaults()
}

func (kb *KeyBinds) Validate() error {
	var merr *multierror.Error

	merr = multierror.Append(merr, keys.ValidateBinds(
		kb.Common.GetKeyBinds(),
		kb.List.GetKeyBinds(),
	))
	merr = multierror.Append(merr, keys.ValidateBinds(
		kb.Common.GetKeyBinds(),
		kb.Pager.GetKeyBinds(),
	))

	return merr.ErrorOrNil()
}

type CommonKeyBinds struct {
	Quit    *keys.KeyBind `yaml:"quit"`
	Suspend *keys.KeyBind `yaml:"suspend"`
	Reload  *keys.KeyBind `yaml:"reload"`
	Help    *keys.KeyBind `yaml:"help"`
	Error   *keys.KeyBind `yaml:"error"`
	Escape  *keys.KeyBind `yaml:"escape"`

	// Navigation.
	Up    *keys.KeyBind `yaml:"up"`
	Down  *keys.KeyBind `yaml:"down"`
	Left  *keys.KeyBind `yaml:"left"`
	Right *keys.KeyBind `yaml:"right"`
	Prev  *keys.KeyBind `yaml:"prev"`
	Next  *keys.KeyBind `yaml:"next"`
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

type ListKeyBinds struct {
	Open     *keys.KeyBind `yaml:"open"`
	Find     *keys.KeyBind `yaml:"find"`
	Home     *keys.KeyBind `yaml:"home"`
	End      *keys.KeyBind `yaml:"end"`
	PageUp   *keys.KeyBind `yaml:"pageUp"`
	PageDown *keys.KeyBind `yaml:"pageDown"`
}

func (kb *ListKeyBinds) EnsureDefaults() {
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

func (kb *ListKeyBinds) GetKeyBinds() []keys.KeyBind {
	return []keys.KeyBind{
		*kb.Open,
		*kb.Find,
		*kb.Home,
		*kb.End,
		*kb.PageUp,
		*kb.PageDown,
	}
}

type PagerKeyBinds struct {
	Copy *keys.KeyBind

	// Navigation.
	Home         *keys.KeyBind `yaml:"home"`
	End          *keys.KeyBind `yaml:"end"`
	PageUp       *keys.KeyBind `yaml:"pageUp"`
	PageDown     *keys.KeyBind `yaml:"pageDown"`
	HalfPageUp   *keys.KeyBind `yaml:"halfPageUp"`
	HalfPageDown *keys.KeyBind `yaml:"halfPageDown"`
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
