package rule

import (
	"errors"
	"fmt"

	"github.com/google/cel-go/cel"

	"github.com/macropower/kat/pkg/expr"
	"github.com/macropower/kat/pkg/profile"
)

// Rule uses a CEL matcher to determine if its profile should be applied.
//
// CEL expressions have access to variables:
//   - `files` (list<string>): All file paths in directory
//   - `dir` (string): The directory path being processed
//
// CEL expressions must return a boolean value:
//   - files.exists(f, pathExt(f) in [".yaml", ".yml"]) - true if any YAML files exist
//   - files.exists(f, pathBase(f) in ["Chart.yaml", "values.yaml"]) - true if Chart or values files exist
//   - files.exists(f, pathBase(f) == "Chart.yaml") - true if Chart.yaml exists
//   - files.exists(f, !pathBase(f).matches(".*test.*")) - true if non-test files exist
//   - files.exists(f, pathBase(f) == "Chart.yaml" && yamlPath(f, "$.apiVersion") == "v2") - true if Helm v2 charts exist
//   - false - rule doesn't match
//
// CEL path functions available:
//   - pathBase(string): Returns the last element of the path (filename)
//   - pathDir(string): Returns all but the last element of the path (directory)
//   - pathExt(string): Returns the file extension including the dot
//   - yamlPath(file, path): Reads a YAML file and extracts value at path (returns null if not found)
//
// CEL also provides standard functions like `endsWith`, `contains`,
// `startsWith`, `matches`, along with list functions like `filter`, `exists`, `in`, and
// logical operators like `&&`, `||`, and `!`.
//
// Use the `in` operator to check membership in lists, e.g.: pathBase(f) in ["Chart.yaml"].
type Rule struct {
	matchProgram cel.Program      // Compiled CEL program for matching file paths.
	pfl          *profile.Profile // Profile associated with the rule.

	// Match is a CEL expression to match file paths.
	Match string `json:"match" jsonschema:"title=Match Expression"`
	// Profile is the name of the profile to use when this rule matches.
	Profile string `json:"profile" jsonschema:"title=Profile Name"`
}

// New creates a new rule with the given profile name and match expression.
func New(profileName, match string) (*Rule, error) {
	r := &Rule{
		Match:   match,
		Profile: profileName,
	}
	if err := r.CompileMatch(); err != nil {
		return nil, fmt.Errorf("rule %q: %w", match, err)
	}

	return r, nil
}

// MustNew creates a new rule and panics if there's an error.
func MustNew(profileName, match string) *Rule {
	r, err := New(profileName, match)
	if err != nil {
		panic(err)
	}

	return r
}

// CompileMatch compiles the rule's match expression into a CEL program.
func (r *Rule) CompileMatch() error {
	if r.matchProgram == nil {
		env, err := expr.CreateEnvironment()
		if err != nil {
			return fmt.Errorf("create CEL environment: %w", err)
		}

		ast, issues := env.Compile(r.Match)
		if issues != nil && issues.Err() != nil {
			return fmt.Errorf("compile match expression: %w", issues.Err())
		}

		program, err := env.Program(ast)
		if err != nil {
			return fmt.Errorf("create CEL program: %w", err)
		}

		r.matchProgram = program
	}

	return nil
}

// MatchFiles evaluates the rule against a collection of files in a directory.
// This allows CEL expressions to operate on the entire file collection and
// return a boolean result.
//
// The CEL expression must return a boolean value indicating whether the rule matches.
func (r *Rule) MatchFiles(dirPath string, files []string) bool {
	if r.matchProgram == nil {
		panic(errors.New("rule missing a match expression"))
	}

	result, _, err := r.matchProgram.Eval(map[string]any{
		"files": files,
		"dir":   dirPath,
	})
	if err != nil {
		// If evaluation fails, consider it a non-match.
		return false
	}

	// CEL expression must return a boolean value.
	if boolVal, ok := result.Value().(bool); ok {
		return boolVal
	}

	// If the result is not a boolean, treat as non-match.
	return false
}

func (r *Rule) GetProfile() *profile.Profile {
	if r.pfl == nil {
		panic(errors.New("rule missing a profile"))
	}

	return r.pfl
}

func (r *Rule) SetProfile(p *profile.Profile) {
	r.pfl = p
}

func (r *Rule) String() string {
	p := r.GetProfile()

	return fmt.Sprintf("%s: %s", r.Profile, p.Command.String())
}
