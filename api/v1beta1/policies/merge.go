package policies

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	"go.jacobcolvin.com/niceyaml"
	"go.jacobcolvin.com/niceyaml/paths"
)

// mergeRootFromValue parses YAML data, merges a value at the root,
// and returns the result. Comments and structure in the original data are preserved.
func mergeRootFromValue(data []byte, v any) ([]byte, error) {
	file, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	node, err := yaml.ValueToNode(v, niceyaml.PrettyEncoderOptions...)
	if err != nil {
		return nil, fmt.Errorf("convert value to node: %w", err)
	}

	rootPath := paths.Root().Path()
	err = rootPath.MergeFromNode(file, node)
	if err != nil {
		return nil, fmt.Errorf("merge yaml: %w", err)
	}

	return []byte(file.String()), nil
}
