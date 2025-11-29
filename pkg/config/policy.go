package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/invopop/jsonschema"

	_ "embed"

	"github.com/macropower/kat/pkg/yaml"
)

//go:generate go run ../../internal/schemagen/policy/main.go -o policies.v1beta1.json

var (
	//go:embed policy.yaml
	defaultPolicyYAML []byte

	//go:embed policies.v1beta1.json
	policySchemaJSON []byte

	// ValidPolicyKinds contains the valid kind values for policy configurations.
	ValidPolicyKinds = []string{
		"Policy",
	}

	// PolicyValidator validates policy configuration against the JSON schema.
	PolicyValidator = yaml.MustNewValidator("/policies.v1beta1.json", policySchemaJSON)
)

// ProjectsPolicyConfig controls handling of project-specific configurations (katrc files).
type ProjectsPolicyConfig struct {
	// Trust contains a list of trusted projects.
	// Projects in this list will have their configs loaded without prompting.
	// NOTE: You can also use `--trust` or `--no-trust` flags to control this behavior.
	Trust []*TrustedProject `json:"trust,omitempty" jsonschema:"title=Trust"`
}

// EnsureDefaults initializes nil fields to their default values.
func (c *ProjectsPolicyConfig) EnsureDefaults() {
	if c.Trust == nil {
		c.Trust = []*TrustedProject{}
	}
}

// Policy represents the policy configuration file.
//
//nolint:recvcheck // Must satisfy the jsonschema interface.
type Policy struct {
	// Projects controls handling of project-specific configurations (katrc files).
	Projects *ProjectsPolicyConfig `json:"projects,omitempty" jsonschema:"title=Projects"`
	// APIVersion specifies the API version for this configuration.
	APIVersion string `json:"apiVersion" jsonschema:"title=API Version"`
	// Kind defines the type of configuration.
	Kind string `json:"kind" jsonschema:"title=Kind"`
}

// NewPolicy creates a new [Policy] with default values.
func NewPolicy() *Policy {
	p := &Policy{
		APIVersion: "kat.jacobcolvin.com/v1beta1",
		Kind:       "Policy",
	}
	p.EnsureDefaults()

	return p
}

// EnsureDefaults initializes nil fields to their default values.
func (p *Policy) EnsureDefaults() {
	if p.Projects == nil {
		p.Projects = &ProjectsPolicyConfig{}
	}

	p.Projects.EnsureDefaults()
}

// IsTrusted checks if a project path is in the trusted list.
func (p *Policy) IsTrusted(projectPath string) bool {
	if p.Projects == nil {
		return false
	}

	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return false
	}

	cleanPath := filepath.Clean(absPath)

	for _, tp := range p.Projects.Trust {
		trustedClean := filepath.Clean(tp.Path)
		if cleanPath == trustedClean {
			return true
		}
	}

	return false
}

// TrustProject adds a project to the trust list and persists it to the policy file.
// This function preserves comments and structure in the policy file.
func (p *Policy) TrustProject(projectPath, policyPath string) error {
	if p.Projects == nil {
		p.Projects = &ProjectsPolicyConfig{}
	}

	p.Projects.EnsureDefaults()

	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		absPath = projectPath
	}

	cleanPath := filepath.Clean(absPath)

	// Check for duplicates.
	for _, tp := range p.Projects.Trust {
		if filepath.Clean(tp.Path) == cleanPath {
			return nil // Already trusted.
		}
	}

	p.Projects.Trust = append(p.Projects.Trust, &TrustedProject{Path: cleanPath})

	// Read the existing policy file.
	data, err := readConfig(policyPath)
	if err != nil {
		return fmt.Errorf("read policy: %w", err)
	}

	// Merge the projects section into the policy, preserving comments.
	projectsUpdate := struct {
		Projects *ProjectsPolicyConfig `json:"projects"`
	}{
		Projects: p.Projects,
	}

	merged, err := yaml.MergeRootFromValue(data, projectsUpdate)
	if err != nil {
		return fmt.Errorf("merge projects section: %w", err)
	}

	// Write the modified file back.
	err = os.WriteFile(policyPath, merged, 0o600)
	if err != nil {
		return fmt.Errorf("write policy: %w", err)
	}

	return nil
}

func (p Policy) JSONSchemaExtend(jss *jsonschema.Schema) {
	extendSchemaWithEnums(jss, ValidAPIVersions, ValidPolicyKinds)
}

// MarshalYAML serializes the policy to YAML.
func (p Policy) MarshalYAML() ([]byte, error) {
	type alias Policy

	b := &bytes.Buffer{}

	enc := yaml.NewEncoder(b)
	err := enc.Encode(alias(p))
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}

	defer func() {
		err := enc.Close()
		if err != nil {
			slog.Error("failed to close YAML encoder", slog.Any("error", err))
		}
	}()

	return b.Bytes(), nil
}

// Write writes the policy to the specified path if it doesn't already exist.
func (p Policy) Write(path string) error {
	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		if err == nil && pathInfo.Mode().IsRegular() {
			return nil // Policy already exists.
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

	b, err := p.MarshalYAML()
	if err != nil {
		return err
	}

	err = os.WriteFile(path, b, 0o600)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// PolicyLoader loads and validates policy configuration files.
type PolicyLoader struct {
	*baseLoader
}

// NewPolicyLoaderFromBytes creates a [PolicyLoader] from byte data.
func NewPolicyLoaderFromBytes(data []byte, opts ...ConfigLoaderOpt) *PolicyLoader {
	return &PolicyLoader{baseLoader: newBaseLoader(data, PolicyValidator, opts...)}
}

// NewPolicyLoaderFromFile creates a [PolicyLoader] from a file path.
func NewPolicyLoaderFromFile(path string, opts ...ConfigLoaderOpt) (*PolicyLoader, error) {
	data, err := readConfig(path)
	if err != nil {
		return nil, fmt.Errorf("read policy file: %w", err)
	}

	return &PolicyLoader{
		baseLoader: newBaseLoader(data, PolicyValidator, opts...),
	}, nil
}

// Load parses and returns the Policy.
func (pl *PolicyLoader) Load() (*Policy, error) {
	p := &Policy{}
	dec := yaml.NewDecoder(bytes.NewReader(pl.data))

	err := dec.Decode(p)
	if err != nil {
		return nil, pl.yamlError.Wrap(err)
	}

	p.EnsureDefaults()

	return p, nil
}

// WriteDefaultPolicy writes the embedded default policy.yaml to the specified path.
func WriteDefaultPolicy(path string, force bool) error {
	policyExists := false

	pathInfo, err := os.Stat(path)
	if pathInfo != nil {
		switch {
		case err == nil && pathInfo.Mode().IsRegular():
			policyExists = true
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

	if policyExists && force {
		backupFile := fmt.Sprintf("%s.%d.old", filepath.Base(path), time.Now().UnixNano())
		backupPath := filepath.Join(filepath.Dir(path), backupFile)
		slog.Info("backing up existing policy file", slog.String("path", backupPath))

		err = os.Rename(path, backupPath)
		if err != nil {
			return fmt.Errorf("rename existing policy file to backup: %w", err)
		}

		policyExists = false
	}

	if !policyExists {
		slog.Info("write default policy", slog.String("path", path))

		err = os.WriteFile(path, defaultPolicyYAML, 0o600)
		if err != nil {
			return fmt.Errorf("write policy file: %w", err)
		}
	} else {
		slog.Debug("policy file already exists, skipping write", slog.String("path", path))
	}

	return nil
}

// GetPolicyPath returns the path to the policy configuration file.
func GetPolicyPath() string {
	if xdgHome, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdgHome != "" {
		return filepath.Join(xdgHome, "kat", "policy.yaml")
	}

	usrHome, err := os.UserHomeDir()
	if err == nil && usrHome != "" {
		return filepath.Join(usrHome, ".config", "kat", "policy.yaml")
	}

	tmpPolicy := filepath.Join(os.TempDir(), "kat", "policy.yaml")

	slog.Warn("could not determine user config directory, using temp path for policy",
		slog.String("path", tmpPolicy),
		slog.Any("error", fmt.Errorf("$XDG_CONFIG_HOME is unset, fall back to home directory: %w", err)),
	)

	return tmpPolicy
}
