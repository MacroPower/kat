package yaml

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
)

// MergeRootFromValue parses YAML data, merges a value at the root,
// and returns the result. Comments and structure in the original data are preserved.
func MergeRootFromValue(data []byte, v any) ([]byte, error) {
	file, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	node, err := yaml.ValueToNode(v, DefaultEncoderOptions...)
	if err != nil {
		return nil, fmt.Errorf("convert value to node: %w", err)
	}

	rootPath := NewPathBuilder().Root().Build()
	err = rootPath.MergeFromNode(file, node)
	if err != nil {
		return nil, fmt.Errorf("merge yaml: %w", err)
	}

	return []byte(file.String()), nil
}
