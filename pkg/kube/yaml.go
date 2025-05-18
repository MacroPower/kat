// Copyright 2017-2018 The Argo Authors
// Modifications Copyright 2024-2025 Jacob Colvin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Source:
// https://github.com/argoproj/gitops-engine/blob/54992bf42431e71f71f11647e82105530e56305e/pkg/utils/kube/kube.go#L304-L346

package kube

import (
	"bytes"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

var (
	ErrInvalidYAML         = errors.New("invalid yaml")
	ErrInvalidKubeResource = errors.New("invalid kubernetes resource")
)

type Resource struct {
	YAML   string
	Object *unstructured.Unstructured
}

// SplitYAML splits a YAML file into unstructured objects. Returns list of all unstructured objects
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
func SplitYAML(yamlData []byte) ([]*Resource, error) {
	objs := []*Resource{}

	ymls := splitYAMLToString(yamlData)

	for _, yml := range ymls {
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(yml), u); err != nil {
			return objs, fmt.Errorf("%w: %w", ErrInvalidKubeResource, err)
		}

		objs = append(objs, &Resource{
			YAML:   yml,
			Object: u,
		})
	}

	return objs, nil
}

// splitYAMLToString splits a YAML file into strings without validating or re-encoding.
// It preserves the original document content exactly as provided.
func splitYAMLToString(yamlData []byte) []string {
	if len(yamlData) == 0 {
		return nil
	}

	// Remove leading/trailing empty documents that wouldn't be captured by split.
	yamlData, _ = bytes.CutPrefix(yamlData, []byte("---\n"))
	yamlData, _ = bytes.CutSuffix(yamlData, []byte("\n---"))

	// Use the yaml document separator to split.
	docs := bytes.Split(yamlData, []byte("\n---\n"))

	// Convert to strings and filter empty documents.
	var result []string
	for _, doc := range docs {
		trimmed := bytes.TrimSpace(doc)
		if len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
			result = append(result, string(trimmed))
		}
	}

	return result
}
