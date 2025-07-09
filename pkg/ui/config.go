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
	// KeyBinds contains key binding configurations for different UI components.
	KeyBinds *KeyBinds `json:"keybinds,omitempty" jsonschema:"title=Key Binds"`
	// Themes defines custom color for the UI, defined as a map of theme name to theme config.
	// Themes can be referenced by name in the global and/or profile UI configs.
	Themes map[string]ThemeConfig `json:"themes,omitempty" jsonschema:"title=Themes"`
	// UI contains general UI display settings.
	UI *UIConfig `json:"ui,omitempty" jsonschema:"title=UI"`
}

func NewConfig() *Config {
	c := &Config{}
	c.EnsureDefaults()

	return c
}

type UIConfig struct {
	// MinimumDelay specifies the minimum delay before updating the display.
	MinimumDelay *time.Duration `json:"minimumDelay,omitempty" jsonschema:"title=Minimum Delay,type=string,default=200ms"`
	// Compact enables compact display mode with reduced spacing.
	Compact *bool `json:"compact,omitempty" jsonschema:"title=Enable Compact Display,default=false"`
	// WordWrap enables automatic word wrapping for long text.
	WordWrap *bool `json:"wordWrap,omitempty" jsonschema:"title=Enable Word Wrap,default=true"`
	// ChromaRendering enables syntax highlighting using Chroma.
	ChromaRendering *bool `json:"chromaRendering,omitempty" jsonschema:"title=Enable Chroma Rendering,default=true"`
	// LineNumbers enables line numbers in the display.
	LineNumbers *bool `json:"lineNumbers,omitempty" jsonschema:"title=Enable Line Numbers,default=true"`
	// Theme specifies the theme name to use. This can be a custom theme added under `themes`,
	// or a theme from the Chroma Style Gallery: https://xyproto.github.io/splash/docs/
	Theme string `json:"theme,omitempty" jsonschema:"title=Theme Name"`
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

	minimumDelay.Pattern = `^([1-9]\d{0,2}ms|[1-9]\d{0,5}us|[1-9]\d{0,8}ns)$`
	schema.Properties.Set("minimumDelay", minimumDelay)
}

// ThemeConfig defines custom theme configuration.
type ThemeConfig struct {
	// Styles contains the style entries for Chroma rendering, which uses the same syntax as Pygments.
	// Define a map of Pygments Tokens (https://pygments.org/docs/tokens/)
	// to Pygments Styles (http://pygments.org/docs/styles/).
	Styles chroma.StyleEntries `json:"styles,omitempty" jsonschema:"title=Styles"`
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

// KeyBinds contains key binding configurations for different UI components.
type KeyBinds struct {
	// Common contains key bindings that apply across all UI components.
	Common *common.KeyBinds `json:"common,omitempty" jsonschema:"title=Common Key Binds"`
	// List contains key bindings specific to list views.
	List *list.KeyBinds `json:"list,omitempty" jsonschema:"title=List Key Binds"`
	// Pager contains key bindings specific to the pager view.
	Pager *pager.KeyBinds `json:"pager,omitempty" jsonschema:"title=Pager Key Binds"`
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
