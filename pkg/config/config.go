package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/invopop/jsonschema"

	_ "embed"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/ui"
	"github.com/macropower/kat/pkg/ui/theme"
	"github.com/macropower/kat/pkg/yaml"
)

//go:generate go run ../../internal/schemagen/main.go -o config.v1beta1.json

var (
	//go:embed config.yaml
	defaultConfigYAML []byte

	//go:embed config.v1beta1.json
	schemaJSON []byte

	ValidAPIVersions = []string{
		"kat.jacobcolvin.com/v1beta1",
	}
	ValidKinds = []string{
		"Configuration",
	}

	DefaultValidator = yaml.MustNewValidator("/config.v1beta1.json", schemaJSON)
)

//nolint:recvcheck // Must satisfy the jsonschema interface.
type Config struct {
	Command *command.Config `json:",inline"`
	UI      *ui.Config      `json:",inline"`
	// APIVersion specifies the API version for this configuration.
	APIVersion string `json:"apiVersion" jsonschema:"title=API Version"`
	// Kind defines the type of configuration.
	Kind string `json:"kind" jsonschema:"title=Kind"`
}

func NewConfig() *Config {
	c := &Config{
		APIVersion: "kat.jacobcolvin.com/v1beta1",
		Kind:       "Configuration",
	}
	c.EnsureDefaults()

	return c
}

func (c *Config) EnsureDefaults() {
	if c.UI == nil {
		c.UI = ui.DefaultConfig
	} else {
		c.UI.EnsureDefaults()
	}

	if c.Command == nil {
		c.Command = command.DefaultConfig
	} else {
		c.Command.EnsureDefaults()
	}
}

func (c Config) JSONSchemaExtend(jss *jsonschema.Schema) {
	apiVersion, ok := jss.Properties.Get("apiVersion")
	if !ok {
		panic("apiVersion property not found in schema")
	}

	for _, version := range ValidAPIVersions {
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

	for _, kindValue := range ValidKinds {
		kind.OneOf = append(kind.OneOf, &jsonschema.Schema{
			Type:  "string",
			Const: kindValue,
			Title: "Kind",
		})
	}

	_, _ = jss.Properties.Set("kind", kind)
}

func (c *Config) MarshalYAML() ([]byte, error) {
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b)
	err := enc.Encode(*c)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}

	return b.Bytes(), nil
}

func (c Config) Write(path string) error {
	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		if err == nil && pathInfo.Mode().IsRegular() {
			return nil // Config already exists.
		}
		if pathInfo.IsDir() {
			return fmt.Errorf("%s: path is a directory", path)
		}

		return fmt.Errorf("%s: unknown file state", path)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	b, err := c.MarshalYAML()
	if err != nil {
		return err
	}

	err = os.WriteFile(path, b, 0o600)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

type ConfigValidator interface {
	Validate(data any) error
}

type ConfigLoader struct {
	cv        ConfigValidator
	theme     *theme.Theme
	yamlError *yaml.ErrorWrapper
	data      []byte
}

func NewConfigLoaderFromBytes(data []byte, opts ...ConfigLoaderOpt) *ConfigLoader {
	cl := &ConfigLoader{
		cv:    DefaultValidator,
		theme: theme.Default,
		data:  data,
	}
	for _, opt := range opts {
		opt(cl)
	}

	cl.yamlError = yaml.NewErrorWrapper(
		yaml.WithTheme(cl.theme),
		yaml.WithSource(cl.data),
		yaml.WithSourceLines(4),
	)

	return cl
}

func NewConfigLoaderFromFile(path string, opts ...ConfigLoaderOpt) (*ConfigLoader, error) {
	data, err := readConfig(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cl := NewConfigLoaderFromBytes(data, opts...)

	return cl, nil
}

type ConfigLoaderOpt func(*ConfigLoader)

func WithConfigValidator(cv ConfigValidator) ConfigLoaderOpt {
	return func(cl *ConfigLoader) {
		cl.cv = cv
	}
}

func WithThemeFromData() ConfigLoaderOpt {
	return func(cl *ConfigLoader) {
		cl.theme = getTheme(cl.data)
	}
}

// Validate validates configuration data with [ConfigValidator] without loading
// it into a [Config] struct.
func (cl *ConfigLoader) Validate() error {
	// Decode into interface{} for initial validation.
	var anyConfig any

	dec := yaml.NewDecoder(bytes.NewReader(cl.data))
	err := dec.Decode(&anyConfig)
	if err != nil {
		return cl.yamlError.Wrap(err)
	}

	err = cl.cv.Validate(anyConfig)
	if err != nil {
		return cl.yamlError.Wrap(err)
	}

	return nil
}

func (cl *ConfigLoader) Load() (*Config, error) {
	c := &Config{}
	dec := yaml.NewDecoder(bytes.NewReader(cl.data))
	err := dec.Decode(c)
	if err != nil {
		return nil, cl.yamlError.Wrap(err)
	}

	c.EnsureDefaults()

	// Run Go validation on the config (for requirements that can't be represented in the schema).
	err = c.Command.Validate()
	if err != nil {
		return nil, cl.yamlError.Wrap(err)
	}

	return c, nil
}

func (cl *ConfigLoader) GetTheme() *theme.Theme {
	return cl.theme
}

// WriteDefaultConfig writes the embedded default config.yaml and jsonschema to
// the specified path.
func WriteDefaultConfig(path string, force bool) error {
	configExists := false
	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		switch {
		case err == nil && pathInfo.Mode().IsRegular():
			configExists = true
		case pathInfo.IsDir():
			return fmt.Errorf("%s: path is a directory", path)
		default:
			return fmt.Errorf("%s: unknown file state", path)
		}
	}

	err = os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	if configExists && force {
		// Move the existing file to a backup.
		backupFile := fmt.Sprintf("%s.%d.old", filepath.Base(path), time.Now().UnixNano())
		backupPath := filepath.Join(filepath.Dir(path), backupFile)
		slog.Info("backing up existing config file",
			slog.String("path", backupPath),
		)

		err = os.Rename(path, backupPath)
		if err != nil {
			return fmt.Errorf("rename existing config file to backup: %w", err)
		}

		configExists = false
	}

	// Write the default config file.
	if !configExists {
		slog.Info("write default configuration",
			slog.String("path", path),
		)

		err = os.WriteFile(path, defaultConfigYAML, 0o600)
		if err != nil {
			return fmt.Errorf("write config file: %w", err)
		}
	} else {
		slog.Debug("configuration file already exists, skipping write",
			slog.String("path", path),
		)
	}

	// Write the JSON schema file alongside the config file.
	schemaPath := filepath.Join(filepath.Dir(path), "config.v1beta1.json")
	slog.Debug("write JSON schema",
		slog.String("path", schemaPath),
	)

	err = os.WriteFile(schemaPath, schemaJSON, 0o600)
	if err != nil {
		return fmt.Errorf("write schema file: %w", err)
	}

	return nil
}

func GetPath() string {
	if xdgHome, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdgHome != "" {
		return filepath.Join(xdgHome, "kat", "config.yaml")
	}

	usrHome, err := os.UserHomeDir()
	if err == nil && usrHome != "" {
		return filepath.Join(usrHome, ".config", "kat", "config.yaml")
	}

	tmpConfig := filepath.Join(os.TempDir(), "kat", "config.yaml")

	slog.Warn("could not determine user config directory, using temp path for config",
		slog.String("path", tmpConfig),
		slog.Any("error", fmt.Errorf("$XDG_CONFIG_HOME is unset, fall back to home directory: %w", err)),
	)

	return tmpConfig
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
