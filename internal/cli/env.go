package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// bindEnvVars automatically binds environment variables to cobra command flags.
// Environment variable names are generated as KAT_<FLAG_NAME> where the flag name
// is converted to uppercase and dashes are replaced with underscores.
//
// For example:
//   - Flag "log-level" becomes environment variable "KAT_LOG_LEVEL"
//   - Flag "config" becomes environment variable "KAT_CONFIG"
//
// Arguments take precedence over environment variables, which take precedence
// over default values.
//
// This function also updates flag usage descriptions to include the environment
// variable name, making it visible in help output.
func bindEnvVars(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		bindFlagToEnv(flag)
	})

	cmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		bindFlagToEnv(flag)
	})
}

// bindFlagToEnv binds a single flag to its corresponding environment variable.
func bindFlagToEnv(flag *pflag.Flag) {
	envName := flagToEnvName(flag.Name)

	// Update the flag usage to include the environment variable name.
	if !strings.Contains(flag.Usage, envName) {
		flag.Usage = fmt.Sprintf("%s ($%s)", flag.Usage, envName)
	}

	// Skip if flag was already set via command line arguments.
	if flag.Changed {
		return
	}

	envValue, ok := os.LookupEnv(envName)
	if ok {
		err := flag.Value.Set(envValue)
		if err != nil {
			// Log error but don't fail - use default value instead.
			slog.Error("failed to set flag from environment variable",
				slog.String("flag", flag.Name),
				slog.String("env", envName),
				slog.String("value", envValue),
				slog.Any("error", err),
			)
		}
	}
}

// flagToEnvName converts a flag name to its corresponding environment variable name.
// Example: "log-level" -> "KAT_LOG_LEVEL".
func flagToEnvName(flagName string) string {
	envName := strings.ReplaceAll(flagName, "-", "_")
	return strings.ToUpper(cmdName + "_" + envName)
}
