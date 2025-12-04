package uitest

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// AssertGolden compares output against a golden file, preserving ANSI codes.
// Use -update-golden flag to regenerate golden files.
func AssertGolden(t *testing.T, name, got string) {
	t.Helper()

	goldenPath := filepath.Join("testdata", "golden", name+".golden")

	if *updateGolden {
		err := os.MkdirAll(filepath.Dir(goldenPath), 0o700)
		require.NoError(t, err)

		err = os.WriteFile(goldenPath, []byte(got), 0o600)
		require.NoError(t, err)

		return
	}

	want, err := os.ReadFile(goldenPath) //nolint:gosec // Path constructed from test name.
	require.NoError(t, err, "golden file not found, run with -update-golden to create")
	require.Equal(t, string(want), got)
}

// GoldenPath returns the path to a golden file.
func GoldenPath(name string) string {
	return filepath.Join("testdata", "golden", name+".golden")
}
