package config

import (
	"bytes"
	"log/slog"
	"regexp"
	"strings"

	"github.com/macropower/kat/api"
	"github.com/macropower/kat/api/v1beta1"
	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/yaml"
)

// Validator validates configuration data against a schema.
type Validator interface {
	Validate(data any) error
}

// LoaderOpt configures a [Loader].
type LoaderOpt func(*loaderOptions)

type loaderOptions struct {
	validator    Validator
	extractTheme bool
}

// WithValidator sets a custom validator.
func WithValidator(v Validator) LoaderOpt {
	return func(o *loaderOptions) {
		o.validator = v
	}
}

// WithThemeFromData extracts the theme from the config data for error formatting.
func WithThemeFromData() LoaderOpt {
	return func(o *loaderOptions) {
		o.extractTheme = true
	}
}

// Loader is a generic configuration loader that handles validation,
// YAML parsing, and error formatting for any config type T.
type Loader[T v1beta1.Object] struct {
	validator Validator
	newFunc   func() T
	theme     *theme.Theme
	yamlError *yaml.ErrorWrapper
	data      []byte
}

// NewLoaderFromBytes creates a [Loader] from byte data.
// The newFunc parameter is the constructor for type T (e.g., config.New).
func NewLoaderFromBytes[T v1beta1.Object](
	data []byte,
	newFunc func() T,
	defaultValidator Validator,
	opts ...LoaderOpt,
) *Loader[T] {
	options := &loaderOptions{
		validator: defaultValidator,
	}
	for _, opt := range opts {
		opt(options)
	}

	t := theme.Default
	if options.extractTheme {
		t = getTheme(data)
	}

	return &Loader[T]{
		data:      data,
		newFunc:   newFunc,
		validator: options.validator,
		theme:     t,
		yamlError: yaml.NewErrorWrapper(
			yaml.WithTheme(t),
			yaml.WithSource(data),
			yaml.WithSourceLines(4),
		),
	}
}

// NewLoaderFromFile creates a [Loader] from a file path.
func NewLoaderFromFile[T v1beta1.Object](
	path string,
	newFunc func() T,
	defaultValidator Validator,
	opts ...LoaderOpt,
) (*Loader[T], error) {
	data, err := api.ReadFile(path)
	if err != nil {
		return nil, err //nolint:wrapcheck // Return the original error.
	}

	return NewLoaderFromBytes(data, newFunc, defaultValidator, opts...), nil
}

// Validate validates the configuration data against the schema.
func (l *Loader[T]) Validate() error {
	var anyConfig any

	dec := yaml.NewDecoder(bytes.NewReader(l.data))

	err := dec.Decode(&anyConfig)
	if err != nil {
		return l.yamlError.Wrap(err)
	}

	if l.validator != nil {
		err = l.validator.Validate(anyConfig)
		if err != nil {
			return l.yamlError.Wrap(err)
		}
	}

	return nil
}

// Load parses and returns the configuration.
//
//nolint:ireturn // Generic type parameter return is intentional.
func (l *Loader[T]) Load() (T, error) {
	cfg := l.newFunc()

	dec := yaml.NewDecoder(bytes.NewReader(l.data))
	err := dec.Decode(cfg)
	if err != nil {
		var zero T
		return zero, l.yamlError.Wrap(err)
	}

	cfg.EnsureDefaults()

	return cfg, nil
}

// GetTheme returns the theme for error formatting.
func (l *Loader[T]) GetTheme() *theme.Theme {
	return l.theme
}

func getTheme(data []byte) *theme.Theme {
	var themeName string

	path := yaml.NewPathBuilder().Root().Child("ui").Child("theme").Build()

	err := path.Read(bytes.NewReader(data), &themeName)
	if err == nil {
		return theme.New(themeName)
	}

	slog.Debug("could not read theme, config might be invalid")

	// As a last-ditch effort, try to get the theme using regex.
	// This is a fallback if the config is malformed or missing the theme.
	themeName = extractThemeWithRegex(data)
	if themeName != "" {
		slog.Debug("extracted theme using regex fallback", slog.String("theme", themeName))
		return theme.New(themeName)
	}

	return theme.Default
}

// extractThemeWithRegex attempts to extract the theme from YAML data using regex.
// This is done so that we can style the error output when the config is not valid YAML.
// It looks for the pattern:
//
//	ui:
//	  foo: bar
//	  # ...
//	  theme: <value>
//
// And extracts the theme value.
func extractThemeWithRegex(data []byte) string {
	content := string(data)

	// Pattern explanation:
	// (?m) - multiline mode
	// ^ui:\s*$ - matches "ui:" at start of line with optional whitespace, then end of line
	// ((?:\n[ \t]+.*)*) - captures newlines followed by indented content.
	uiPattern := `(?m)^ui:\s*$((?:\n[ \t]+.*)*)`

	uiRe := regexp.MustCompile(uiPattern)
	uiMatches := uiRe.FindStringSubmatch(content)

	if len(uiMatches) >= 2 {
		uiSection := uiMatches[1]

		// Within the ui section, look for theme: value
		// \n[ \t]+ - newline followed by one or more whitespace characters (indentation)
		// theme:\s* - "theme:" with optional whitespace
		// (?:"([^"#\n]+)"|'([^'#\n]+)'|([^\s#\n]+)) - captures quoted or unquoted theme value.
		themePattern := `\n[ \t]+theme:\s*(?:"([^"#\n]+)"|'([^'#\n]+)'|([^\s#\n]+))`

		themeRe := regexp.MustCompile(themePattern)
		themeMatches := themeRe.FindStringSubmatch(uiSection)

		if len(themeMatches) >= 4 {
			// Check which capture group matched (double quote, single quote, or unquoted).
			for i := 1; i < 4; i++ {
				if themeMatches[i] != "" {
					return strings.TrimSpace(themeMatches[i])
				}
			}
		}
	}

	return ""
}
