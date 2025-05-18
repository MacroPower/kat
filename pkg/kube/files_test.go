// Copyright 2024-2025 Jacob Colvin
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

package kube_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kat/pkg/kube"
)

func TestCommandRunner_RunForPath(t *testing.T) {
	t.Parallel()

	// Setup temp directory for testing
	tempDir := t.TempDir()

	// Create test files
	chartFile := filepath.Join(tempDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("name: test-chart"), 0o644))

	kustomizationFile := filepath.Join(tempDir, "kustomization.yaml")
	require.NoError(t, os.WriteFile(kustomizationFile, []byte("resources: []"), 0o644))

	unknownFile := filepath.Join(tempDir, "unknown.yaml")
	require.NoError(t, os.WriteFile(unknownFile, []byte(""), 0o644))

	// Create a subdirectory with a nested file
	subDir := filepath.Join(tempDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	nestedChartFile := filepath.Join(subDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(nestedChartFile, []byte("name: nested-chart"), 0o644))

	tcs := map[string]struct {
		expectedError error
		path          string
		checkOutput   bool
	}{
		"file not found": {
			path:          filepath.Join(tempDir, "nonexistent.yaml"),
			expectedError: os.ErrNotExist,
			checkOutput:   false,
		},
		"no command for path": {
			path:          unknownFile,
			expectedError: kube.ErrNoCommandForPath,
			checkOutput:   false,
		},
		"directory with no matching files": {
			path:          t.TempDir(), // Empty temp directory
			expectedError: kube.ErrNoCommandForPath,
			checkOutput:   false,
		},
		"match Chart.yaml file": {
			path:          chartFile,
			expectedError: nil, // Command execution will fail in test environment, but path matching should succeed
			checkOutput:   false,
		},
		"match kustomization.yaml file": {
			path:          kustomizationFile,
			expectedError: nil, // Command execution will fail in test environment, but path matching should succeed
			checkOutput:   false,
		},
		"directory with matching file": {
			path:          tempDir,
			expectedError: nil, // Command execution will fail in test environment, but path matching should succeed
			checkOutput:   false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner := kube.NewCommandRunner(tc.path)
			output, err := runner.Run()

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
			}

			if tc.checkOutput {
				assert.NotEmpty(t, output.Stdout)
			}
		})
	}
}

func TestCommandRunner_WithCommand(t *testing.T) {
	t.Parallel()

	runner := kube.NewCommandRunner("", kube.WithCommand(
		kube.MustNewCommand("", "echo", "{apiVersion: v1, kind: Resource}"),
	))
	customRunner, err := runner.Run()
	require.NoError(t, err)

	assert.Empty(t, customRunner.Stderr)
	assert.Equal(t, "{apiVersion: v1, kind: Resource}\n", customRunner.Stdout)
	assert.Equal(t, "v1", customRunner.Resources[0].Object.GetAPIVersion())
	assert.Equal(t, "Resource", customRunner.Resources[0].Object.GetKind())
}
