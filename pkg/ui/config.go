package ui

import (
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/hashicorp/go-multierror"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/list"
	"github.com/macropower/kat/pkg/ui/pager"
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
	Common *common.KeyBinds `yaml:"common"`
	List   *list.KeyBinds   `yaml:"list"`
	Pager  *pager.KeyBinds  `yaml:"pager"`
}

func NewKeyBinds() *KeyBinds {
	kb := &KeyBinds{
		Common: &common.KeyBinds{},
		List:   &list.KeyBinds{},
		Pager:  &pager.KeyBinds{},
	}
	kb.EnsureDefaults()

	return kb
}

func (kb *KeyBinds) EnsureDefaults() {
	if kb.Common == nil {
		kb.Common = &common.KeyBinds{}
	}
	if kb.List == nil {
		kb.List = &list.KeyBinds{}
	}
	if kb.Pager == nil {
		kb.Pager = &pager.KeyBinds{}
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
