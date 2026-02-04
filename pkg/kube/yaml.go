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
	"errors"
	"fmt"

	"go.jacobcolvin.com/niceyaml"
	"go.jacobcolvin.com/niceyaml/lexers"
)

var (
	ErrInvalidYAML         = errors.New("invalid yaml")
	ErrInvalidKubeResource = errors.New("invalid kubernetes resource")
)

type Resource struct {
	Object *Object
	Source *niceyaml.Source
}

// SplitYAML splits a YAML file into unstructured objects. Returns list of all unstructured objects
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
func SplitYAML(yamlData []byte) ([]*Resource, error) {
	var objs []*Resource

	for _, tks := range lexers.TokenizeDocuments(string(yamlData)) {
		source := niceyaml.NewSourceFromTokens(tks)

		dec, err := source.Decoder()
		if err != nil {
			return objs, fmt.Errorf("%w: %w", ErrInvalidYAML, err)
		}

		for _, doc := range dec.Documents() {
			// Skip empty/null documents.
			if doc.Document().Body == nil {
				continue
			}

			obj := &Object{}

			err := doc.Decode(obj)
			if err != nil {
				return objs, fmt.Errorf("%w: %w", ErrInvalidKubeResource, err)
			}

			// Skip objects that decoded to empty (null documents).
			if len(*obj) == 0 {
				continue
			}

			objs = append(objs, &Resource{
				Source: source,
				Object: obj,
			})
		}
	}

	return objs, nil
}
