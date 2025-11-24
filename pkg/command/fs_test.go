package command_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/rule"
)

func TestNewFilteredFS(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupFunc   func() (string, []*rule.Rule, error)
		expectError bool
	}{
		"valid directory with rules": {
			setupFunc: func() (string, []*rule.Rule, error) {
				tmpDir := t.TempDir()
				rules := []*rule.Rule{
					rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
				}

				return tmpDir, rules, nil
			},
			expectError: false,
		},
		"empty rules": {
			setupFunc: func() (string, []*rule.Rule, error) {
				tmpDir := t.TempDir()
				return tmpDir, []*rule.Rule{}, nil
			},
			expectError: false,
		},
		"non-existent directory": {
			setupFunc: func() (string, []*rule.Rule, error) {
				rules := []*rule.Rule{
					rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
				}

				return "/nonexistent/path", rules, nil
			},
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dirPath, rules, setupErr := tc.setupFunc()
			require.NoError(t, setupErr)

			fs, err := command.NewFilteredFSFromPath(dirPath, rules...)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "open directory")
				assert.Nil(t, fs)
			} else {
				require.NoError(t, err)
				require.NotNil(t, fs)

				// Verify we can get the name
				assert.Contains(t, fs.Name(), filepath.Base(dirPath))

				// Clean up
				err = fs.Close()
				require.NoError(t, err)
			}
		})
	}
}

func TestFilteredFS_Close(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	fs, err := command.NewFilteredFSFromPath(tmpDir,
		rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
	)
	require.NoError(t, err)
	require.NotNil(t, fs)

	// Close should work
	require.NoError(t, fs.Close())

	// Multiple closes should not cause issues
	require.NoError(t, fs.Close())
}

func TestFilteredFS_Name(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	fs, err := command.NewFilteredFSFromPath(tmpDir,
		rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
	)
	require.NoError(t, err)
	require.NotNil(t, fs)

	name := fs.Name()
	assert.Contains(t, name, filepath.Base(tmpDir))

	// Name should still work after close
	require.NoError(t, fs.Close())

	name = fs.Name()
	assert.Contains(t, name, filepath.Base(tmpDir))
}

func TestFilteredFS_Open(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		fileName    string
		expectError bool
	}{
		"existing file": {
			fileName:    "test.yaml",
			expectError: false,
		},
		"non-existent file": {
			fileName:    "nonexistent.yaml",
			expectError: true,
		},
		"root directory": {
			fileName:    ".",
			expectError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test.yaml")
			err := os.WriteFile(testFile, []byte("key: value"), 0o644)
			require.NoError(t, err)

			fs, err := command.NewFilteredFSFromPath(tmpDir,
				rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
			)
			require.NoError(t, err)
			require.NotNil(t, fs)

			file, err := fs.Open(tc.fileName)

			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, file)
			} else {
				require.NoError(t, err)
				require.NotNil(t, file)
			}

			require.NoError(t, fs.Close())
		})
	}
}

func TestFilteredFS_OpenFile(t *testing.T) {
	t.Parallel()

	testFile := "test.yaml"

	tests := map[string]struct {
		fileName    string
		flag        int
		perm        os.FileMode
		expectError bool
	}{
		"read existing file": {
			fileName:    testFile,
			flag:        os.O_RDONLY,
			perm:        0o644,
			expectError: false,
		},
		"create new file": {
			fileName:    "new.yaml",
			flag:        os.O_CREATE | os.O_WRONLY,
			perm:        0o644,
			expectError: false,
		},
		"non-existent file read": {
			fileName:    "nonexistent.yaml",
			flag:        os.O_RDONLY,
			perm:        0o644,
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			err := os.WriteFile(filepath.Join(tmpDir, testFile), []byte("key: value"), 0o644)
			require.NoError(t, err)

			fs, err := command.NewFilteredFSFromPath(tmpDir,
				rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
			)
			require.NoError(t, err)
			require.NotNil(t, fs)

			file, err := fs.OpenFile(tc.fileName, tc.flag, tc.perm)

			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, file)
			} else {
				require.NoError(t, err)
				require.NotNil(t, file)
			}

			require.NoError(t, fs.Close())
		})
	}
}

func TestFilteredFS_Stat(t *testing.T) {
	t.Parallel()

	testFile := "test.yaml"

	tests := map[string]struct {
		fileName    string
		expectError bool
	}{
		"existing file": {
			fileName:    testFile,
			expectError: false,
		},
		"root directory": {
			fileName:    ".",
			expectError: false,
		},
		"non-existent file": {
			fileName:    "nonexistent.yaml",
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			err := os.WriteFile(filepath.Join(tmpDir, testFile), []byte("key: value"), 0o644)
			require.NoError(t, err)

			fs, err := command.NewFilteredFSFromPath(tmpDir,
				rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
			)
			require.NoError(t, err)
			require.NotNil(t, fs)

			info, err := fs.Stat(tc.fileName)

			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, info)
			} else {
				require.NoError(t, err)
				require.NotNil(t, info)
			}

			require.NoError(t, fs.Close())
		})
	}
}

func TestFilteredFS_ReadDir(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupFunc     func() (string, []*rule.Rule)
		expectedDirs  []string
		expectedFiles []string
		expectError   bool
	}{
		"yaml files only": {
			setupFunc: func() (string, []*rule.Rule) {
				tmpDir := t.TempDir()

				// Create test files - when YAML files exist, ALL files should be returned
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("key: value"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "values.yml"), []byte("app: test"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0o644))

				rules := []*rule.Rule{
					rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
				}

				return tmpDir, rules
			},
			expectError:   false,
			expectedFiles: []string{"config.yaml", "values.yml"},
		},
		"no matching files": {
			setupFunc: func() (string, []*rule.Rule) {
				tmpDir := t.TempDir()

				// Create non-matching files
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0o644))

				rules := []*rule.Rule{
					rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
				}

				return tmpDir, rules
			},
			expectError:   false,
			expectedFiles: []string{},
		},
		"directory with matching subdirectory": {
			setupFunc: func() (string, []*rule.Rule) {
				tmpDir := t.TempDir()

				// Create subdirectory with yaml file
				subDir := filepath.Join(tmpDir, "config")
				require.NoError(t, os.Mkdir(subDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(subDir, "app.yaml"), []byte("name: test"), 0o644))

				// Create non-matching files in root
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0o644))

				rules := []*rule.Rule{
					rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
				}

				return tmpDir, rules
			},
			expectError:  false,
			expectedDirs: []string{"config"},
		},
		"multiple nested directories": {
			setupFunc: func() (string, []*rule.Rule) {
				tmpDir := t.TempDir()

				// Create nested structure
				configDir := filepath.Join(tmpDir, "config")
				envDir := filepath.Join(configDir, "env")
				require.NoError(t, os.MkdirAll(envDir, 0o755))

				// Add yaml file deep in structure
				require.NoError(t, os.WriteFile(filepath.Join(envDir, "prod.yaml"), []byte("env: production"), 0o644))

				// Add non-matching directory
				docsDir := filepath.Join(tmpDir, "docs")
				require.NoError(t, os.Mkdir(docsDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(docsDir, "README.md"), []byte("# Docs"), 0o644))

				rules := []*rule.Rule{
					rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
				}

				return tmpDir, rules
			},
			expectError:  false,
			expectedDirs: []string{"config"},
		},
		"rule matches directory name": {
			setupFunc: func() (string, []*rule.Rule) {
				tmpDir := t.TempDir()

				// Create files and directories
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0o644))
				require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "helm"), 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(filepath.Join(tmpDir, "helm"), "Chart.yaml"),
					[]byte("apiVersion: v2"),
					0o644,
				))

				rules := []*rule.Rule{
					rule.MustNew("helm", `files.exists(f, pathBase(f) == "Chart.yaml")`),
				}

				return tmpDir, rules
			},
			expectError:  false,
			expectedDirs: []string{"helm"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tmpDir, rules := tc.setupFunc()

			fs, err := command.NewFilteredFSFromPath(tmpDir, rules...)
			require.NoError(t, err)

			require.NotNil(t, fs)

			entries, err := fs.ReadDir(".")

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Check expected files
			var (
				actualFiles []string
				actualDirs  []string
			)

			for _, entry := range entries {
				if entry.IsDir() {
					actualDirs = append(actualDirs, entry.Name())
				} else {
					actualFiles = append(actualFiles, entry.Name())
				}
			}

			if tc.expectedFiles != nil {
				assert.ElementsMatch(t, tc.expectedFiles, actualFiles)
			}
			if tc.expectedDirs != nil {
				assert.ElementsMatch(t, tc.expectedDirs, actualDirs)
			}

			require.NoError(t, fs.Close())
		})
	}
}

func TestFilteredFS_ReadDir_NonExistentDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	fs, err := command.NewFilteredFSFromPath(tmpDir,
		rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
	)
	require.NoError(t, err)
	require.NotNil(t, fs)

	entries, err := fs.ReadDir("nonexistent")
	require.Error(t, err)
	assert.Nil(t, entries)

	require.NoError(t, fs.Close())
}

func TestFilteredFS_MaxDepthLimiting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create deep nested structure: root/level1/level2/.../level15/file.yaml
	currentDir := tmpDir
	for i := 1; i <= 15; i++ {
		currentDir = filepath.Join(currentDir, fmt.Sprintf("level%d", i))
		require.NoError(t, os.Mkdir(currentDir, 0o755))
	}

	// Add yaml file at the deepest level
	require.NoError(t, os.WriteFile(filepath.Join(currentDir, "deep.yaml"), []byte("key: value"), 0o644))

	fs, err := command.NewFilteredFSFromPath(tmpDir,
		rule.MustNew("yaml", `files.exists(f, pathExt(f) in [".yaml", ".yml"])`),
	)
	require.NoError(t, err)
	require.NotNil(t, fs)

	entries, err := fs.ReadDir(".")
	require.NoError(t, err)

	// Check that no entries are returned at the root level, since the match will
	// occur outside of maxDepth.
	assert.Empty(t, entries)

	require.NoError(t, fs.Close())
}
