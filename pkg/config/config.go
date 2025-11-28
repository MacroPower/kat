package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/invopop/jsonschema"

	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/yaml"
)

// ValidAPIVersions contains the valid API versions for all configuration kinds.
var ValidAPIVersions = []string{
	"kat.jacobcolvin.com/v1beta1",
}

// ConfigValidator validates configuration data against a schema.
type ConfigValidator interface {
	Validate(data any) error
}

// baseLoader contains shared loader functionality.
type baseLoader struct {
	cv        ConfigValidator
	theme     *theme.Theme
	yamlError *yaml.ErrorWrapper
	data      []byte
}

// ConfigLoaderOpt configures a loader.
type ConfigLoaderOpt func(*baseLoader)

// WithConfigValidator sets a custom validator.
func WithConfigValidator(cv ConfigValidator) ConfigLoaderOpt {
	return func(bl *baseLoader) {
		bl.cv = cv
	}
}

// WithThemeFromData extracts the theme from the config data.
func WithThemeFromData() ConfigLoaderOpt {
	return func(bl *baseLoader) {
		bl.theme = getTheme(bl.data)
	}
}

func newBaseLoader(data []byte, defaultValidator ConfigValidator, opts ...ConfigLoaderOpt) *baseLoader {
	bl := &baseLoader{
		cv:    defaultValidator,
		theme: theme.Default,
		data:  data,
	}

	for _, opt := range opts {
		opt(bl)
	}

	bl.yamlError = yaml.NewErrorWrapper(
		yaml.WithTheme(bl.theme),
		yaml.WithSource(bl.data),
		yaml.WithSourceLines(4),
	)

	return bl
}

// Validate validates configuration data against the schema.
func (bl *baseLoader) Validate() error {
	var anyConfig any

	dec := yaml.NewDecoder(bytes.NewReader(bl.data))

	err := dec.Decode(&anyConfig)
	if err != nil {
		return bl.yamlError.Wrap(err)
	}

	if bl.cv != nil {
		err = bl.cv.Validate(anyConfig)
		if err != nil {
			return bl.yamlError.Wrap(err)
		}
	}

	return nil
}

// GetTheme returns the theme for error formatting.
func (bl *baseLoader) GetTheme() *theme.Theme {
	return bl.theme
}

// extendSchemaWithEnums adds apiVersion and kind enum constraints to a JSON schema.
func extendSchemaWithEnums(jss *jsonschema.Schema, apiVersions, kinds []string) {
	apiVersion, ok := jss.Properties.Get("apiVersion")
	if !ok {
		panic("apiVersion property not found in schema")
	}

	for _, version := range apiVersions {
		apiVersion.OneOf = append(apiVersion.OneOf, &jsonschema.Schema{
			Type:  "string",
			Const: version,
			Title: "API Version",
		})
	}

	_, _ = jss.Properties.Set("apiVersion", apiVersion)

	kind, ok := jss.Properties.Get("kind")
	if !ok {
		panic("kind property not found in schema")
	}

	for _, kindValue := range kinds {
		kind.OneOf = append(kind.OneOf, &jsonschema.Schema{
			Type:  "string",
			Const: kindValue,
			Title: "Kind",
		})
	}

	_, _ = jss.Properties.Set("kind", kind)
}

func readConfig(path string) ([]byte, error) {
	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		if err == nil && pathInfo.IsDir() {
			return nil, fmt.Errorf("%s: path is a directory", path)
		}
		if err == nil && !pathInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: unknown file state", path)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // G304: Potential file inclusion via variable.
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return data, nil
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

// ExtractThemeWithRegex attempts to extract the theme from YAML data using regex.
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
