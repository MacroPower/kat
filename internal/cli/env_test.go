package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/internal/cli"
)

func TestBindEnvVars(t *testing.T) {
	tcs := map[string]struct {
		envVars       map[string]string
		wantLogLevel  string
		wantLogFormat string
		args          []string
	}{
		"environment variables are bound when no args provided": {
			envVars: map[string]string{
				"KAT_LOG_LEVEL":  "debug",
				"KAT_LOG_FORMAT": "json",
			},
			args:          []string{},
			wantLogLevel:  "debug",
			wantLogFormat: "json",
		},
		"command line args take precedence over environment variables": {
			envVars: map[string]string{
				"KAT_LOG_LEVEL":  "debug",
				"KAT_LOG_FORMAT": "json",
			},
			args:          []string{"--log-level", "error", "--log-format", "text"},
			wantLogLevel:  "error",
			wantLogFormat: "text",
		},
		"partial environment variable override": {
			envVars: map[string]string{
				"KAT_LOG_LEVEL": "warn",
			},
			args:          []string{"--log-format", "json"},
			wantLogLevel:  "warn",
			wantLogFormat: "json",
		},
		"no environment variables uses defaults": {
			envVars:       map[string]string{},
			args:          []string{},
			wantLogLevel:  "info", // Default value.
			wantLogFormat: "text", // Default value.
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			for key, val := range tc.envVars {
				t.Setenv(key, val)
			}

			cmd := cli.NewRootCmd()
			cmd.SetArgs(tc.args)

			// Parse flags (this triggers environment variable binding).
			err := cmd.ParseFlags(tc.args)
			require.NoError(t, err)

			// Check flag values.
			logLevel, err := cmd.Flags().GetString("log-level")
			require.NoError(t, err)
			assert.Equal(t, tc.wantLogLevel, logLevel)

			logFormat, err := cmd.Flags().GetString("log-format")
			require.NoError(t, err)
			assert.Equal(t, tc.wantLogFormat, logFormat)
		})
	}
}

// Test that flag usage strings are updated to include environment variable names.
func TestEnvironmentVariableUsageUpdate(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCmd()

	logLevelFlag := cmd.PersistentFlags().Lookup("log-level")
	require.NotNil(t, logLevelFlag)
	assert.Contains(t, logLevelFlag.Usage, "$KAT_LOG_LEVEL")

	configFlag := cmd.Flags().Lookup("config")
	require.NotNil(t, configFlag)
	assert.Contains(t, configFlag.Usage, "$KAT_CONFIG")
}
