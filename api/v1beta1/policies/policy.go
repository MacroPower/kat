// Package policies provides the Policy configuration type for kat.
package policies

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/invopop/jsonschema"

	_ "embed"

	"github.com/macropower/kat/api"
	"github.com/macropower/kat/api/v1beta1"
	"github.com/macropower/kat/pkg/yaml"
)

//go:generate go run ../../../internal/schemagen/policy/main.go -o policies.v1beta1.json

var (
	//go:embed policy.yaml
	defaultPolicyYAML []byte

	//go:embed policies.v1beta1.json
	policySchemaJSON []byte

	// ValidKinds contains the valid kind values for policy configurations.
	ValidKinds = []string{"Policy"}

	// DefaultValidator validates policy configuration against the JSON schema.
	DefaultValidator = yaml.MustNewValidator("/policies.v1beta1.json", policySchemaJSON)

	// Compile-time interface checks.
	_ v1beta1.Object = (*Policy)(nil)
)

// TrustedProject represents a trusted project.
type TrustedProject struct {
	// Path is the absolute path to a trusted directory.
	Path string `json:"path" jsonschema:"title=Path"`
}

// ProjectsPolicyConfig controls handling of project-specific configurations (.katrc.yaml files).
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
	Projects         *ProjectsPolicyConfig `json:"projects,omitempty" jsonschema:"title=Projects"`
	v1beta1.TypeMeta `json:",inline"`
}

// New creates a new [Policy] with default values.
func New() *Policy {
	p := &Policy{
		TypeMeta: v1beta1.TypeMeta{
			APIVersion: v1beta1.APIVersion,
			Kind:       "Policy",
		},
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
	data, err := api.ReadFile(policyPath)
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
	v1beta1.ExtendSchemaWithEnums(jss, v1beta1.ValidAPIVersions, ValidKinds)
}

// MarshalYAML serializes the policy to YAML.
func (p Policy) MarshalYAML() ([]byte, error) {
	type alias Policy

	b, err := api.MarshalYAML(alias(p))
	if err != nil {
		return nil, fmt.Errorf("marshal policy: %w", err)
	}

	return b, nil
}

// Write writes the policy to the specified path if it doesn't already exist.
func (p Policy) Write(path string) error {
	b, err := p.MarshalYAML()
	if err != nil {
		return err
	}

	err = api.WriteIfNotExists(path, b)
	if err != nil {
		return fmt.Errorf("write policy: %w", err)
	}

	return nil
}

// WriteDefault writes the embedded default policy.yaml to the specified path.
func WriteDefault(path string, force bool) error {
	err := api.WriteDefaultFile(path, defaultPolicyYAML, force, "policy")
	if err != nil {
		return fmt.Errorf("write default policy: %w", err)
	}

	return nil
}

// GetPath returns the path to the policy configuration file.
func GetPath() string {
	return api.GetConfigPath("policy.yaml")
}
