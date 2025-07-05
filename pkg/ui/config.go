package ui

import (
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/invopop/jsonschema"

	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/list"
	"github.com/macropower/kat/pkg/ui/pager"
)

var DefaultConfig = NewConfig()

// Config contains TUI-specific configuration.
type Config struct {
	KeyBinds *KeyBinds              `json:"keybinds,omitempty"`
	Themes   map[string]ThemeConfig `json:"themes,omitempty"`
	UI       *UIConfig              `json:"ui,omitempty"`
}

func NewConfig() *Config {
	c := &Config{}
	c.EnsureDefaults()

	return c
}

type UIConfig struct {
	MinimumDelay    *time.Duration `json:"minimumDelay,omitempty"`
	Compact         *bool          `json:"compact,omitempty"`
	WordWrap        *bool          `json:"wordWrap,omitempty"`
	ChromaRendering *bool          `json:"chromaRendering,omitempty"`
	LineNumbers     *bool          `json:"lineNumbers,omitempty"`
	Theme           string         `json:"theme,omitempty"`
}

func (c *UIConfig) EnsureDefaults() {
	if c.MinimumDelay == nil {
		defaultDelay := 200 * time.Millisecond
		c.MinimumDelay = &defaultDelay
	}
}

func (c UIConfig) JSONSchemaExtend(schema *jsonschema.Schema) {
	minimumDelay, ok := schema.Properties.Get("minimumDelay")
	if !ok {
		panic("minimumDelay property not found in UIConfig schema")
	}
	minimumDelay.Type = "string"
	minimumDelay.Default = "200ms"
	minimumDelay.Pattern = `^([1-9]\d{0,2}ms|[1-9]\d{0,5}us|[1-9]\d{0,8}ns)$`
	schema.Properties.Set("minimumDelay", minimumDelay)
}

type ThemeConfig struct {
	Styles chroma.StyleEntries `json:"styles,omitempty"`
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
	Common *common.KeyBinds `json:"common,omitempty"`
	List   *list.KeyBinds   `json:"list,omitempty"`
	Pager  *pager.KeyBinds  `json:"pager,omitempty"`
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
