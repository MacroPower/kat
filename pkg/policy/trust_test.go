package policy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/api/v1beta1/policies"
	"github.com/macropower/kat/pkg/policy"
)

// mockTrustPrompter is a test implementation of policy.TrustPrompter.
type mockTrustPrompter struct {
	err      error
	decision policy.TrustDecision
}

func (m *mockTrustPrompter) Prompt(_, _ string) (policy.TrustDecision, error) {
	return m.decision, m.err
}

// validRuntimeConfig returns valid runtime config YAML content.
func validRuntimeConfig() string {
	return `apiVersion: kat.jacobcolvin.com/v1beta1
kind: RuntimeConfig
`
}

// setupProjectDir creates a temp project directory with a .katrc.yaml file.
func setupProjectDir(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()

	if content != "" {
		err := os.WriteFile(filepath.Join(dir, ".katrc.yaml"), []byte(content), 0o600)
		require.NoError(t, err)
	}

	return dir
}

// setupPolicyFile creates a policy file in the given directory.
func setupPolicyFile(t *testing.T, dir string) string {
	t.Helper()

	policyPath := filepath.Join(dir, "policy.yaml")
	pol := policies.New()

	b, err := pol.MarshalYAML()
	require.NoError(t, err)

	err = os.WriteFile(policyPath, b, 0o600)
	require.NoError(t, err)

	return policyPath
}

func TestNewTrustManager(t *testing.T) {
	t.Parallel()

	t.Run("with nil policy creates default", func(t *testing.T) {
		t.Parallel()

		policyPath := filepath.Join(t.TempDir(), "policy.yaml")
		tm := policy.NewTrustManager(nil, policyPath)

		assert.NotNil(t, tm)
	})

	t.Run("with provided policy uses it", func(t *testing.T) {
		t.Parallel()

		pol := policies.New()
		policyPath := filepath.Join(t.TempDir(), "policy.yaml")
		tm := policy.NewTrustManager(pol, policyPath)

		assert.NotNil(t, tm)
	})
}

func TestTrustManager_LoadTrustedRuntimeConfig(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupFunc      func(t *testing.T) (targetPath, policyPath string, pol *policies.Policy)
		prompter       policy.TrustPrompter
		errMsg         string
		mode           policy.TrustMode
		wantNil        bool
		wantErr        bool
		checkTrustList bool
	}{
		"no runtime config found": {
			setupFunc: func(t *testing.T) (string, string, *policies.Policy) {
				t.Helper()

				dir := t.TempDir() // Empty directory.
				policyDir := t.TempDir()
				policyPath := setupPolicyFile(t, policyDir)

				return dir, policyPath, policies.New()
			},
			mode:    policy.TrustModePrompt,
			wantNil: true,
			wantErr: false,
		},
		"TrustModeSkip returns nil": {
			setupFunc: func(t *testing.T) (string, string, *policies.Policy) {
				t.Helper()

				dir := setupProjectDir(t, validRuntimeConfig())
				policyDir := t.TempDir()
				policyPath := setupPolicyFile(t, policyDir)

				return dir, policyPath, policies.New()
			},
			mode:    policy.TrustModeSkip,
			wantNil: true,
			wantErr: false,
		},
		"TrustModeAllow saves to policy and returns config": {
			setupFunc: func(t *testing.T) (string, string, *policies.Policy) {
				t.Helper()

				dir := setupProjectDir(t, validRuntimeConfig())
				policyDir := t.TempDir()
				policyPath := setupPolicyFile(t, policyDir)

				return dir, policyPath, policies.New()
			},
			mode:           policy.TrustModeAllow,
			wantNil:        false,
			wantErr:        false,
			checkTrustList: true,
		},
		"TrustModePrompt with already trusted returns config": {
			setupFunc: func(t *testing.T) (string, string, *policies.Policy) {
				t.Helper()

				dir := setupProjectDir(t, validRuntimeConfig())
				policyDir := t.TempDir()
				policyPath := setupPolicyFile(t, policyDir)

				pol := policies.New()
				pol.Projects.Trust = append(pol.Projects.Trust, &policies.TrustedProject{Path: dir})

				return dir, policyPath, pol
			},
			mode:    policy.TrustModePrompt,
			wantNil: false,
			wantErr: false,
		},
		"TrustModePrompt with nil prompter returns nil": {
			setupFunc: func(t *testing.T) (string, string, *policies.Policy) {
				t.Helper()

				dir := setupProjectDir(t, validRuntimeConfig())
				policyDir := t.TempDir()
				policyPath := setupPolicyFile(t, policyDir)

				return dir, policyPath, policies.New()
			},
			prompter: nil,
			mode:     policy.TrustModePrompt,
			wantNil:  true,
			wantErr:  false,
		},
		"TrustModePrompt with ErrNotInteractive returns nil": {
			setupFunc: func(t *testing.T) (string, string, *policies.Policy) {
				t.Helper()

				dir := setupProjectDir(t, validRuntimeConfig())
				policyDir := t.TempDir()
				policyPath := setupPolicyFile(t, policyDir)

				return dir, policyPath, policies.New()
			},
			prompter: &mockTrustPrompter{err: policy.ErrNotInteractive},
			mode:     policy.TrustModePrompt,
			wantNil:  true,
			wantErr:  false,
		},
		"TrustModePrompt with TrustDecisionSkip returns nil": {
			setupFunc: func(t *testing.T) (string, string, *policies.Policy) {
				t.Helper()

				dir := setupProjectDir(t, validRuntimeConfig())
				policyDir := t.TempDir()
				policyPath := setupPolicyFile(t, policyDir)

				return dir, policyPath, policies.New()
			},
			prompter: &mockTrustPrompter{decision: policy.TrustDecisionSkip},
			mode:     policy.TrustModePrompt,
			wantNil:  true,
			wantErr:  false,
		},
		"TrustModePrompt with TrustDecisionAllow saves and returns config": {
			setupFunc: func(t *testing.T) (string, string, *policies.Policy) {
				t.Helper()

				dir := setupProjectDir(t, validRuntimeConfig())
				policyDir := t.TempDir()
				policyPath := setupPolicyFile(t, policyDir)

				return dir, policyPath, policies.New()
			},
			prompter:       &mockTrustPrompter{decision: policy.TrustDecisionAllow},
			mode:           policy.TrustModePrompt,
			wantNil:        false,
			wantErr:        false,
			checkTrustList: true,
		},
		"invalid runtime config returns error": {
			setupFunc: func(t *testing.T) (string, string, *policies.Policy) {
				t.Helper()

				dir := setupProjectDir(t, `invalid: [yaml`)
				policyDir := t.TempDir()
				policyPath := setupPolicyFile(t, policyDir)

				pol := policies.New()
				pol.Projects.Trust = append(pol.Projects.Trust, &policies.TrustedProject{Path: dir})

				return dir, policyPath, pol
			},
			mode:    policy.TrustModePrompt,
			wantErr: true,
			errMsg:  "validate runtime config",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			targetPath, policyPath, pol := tc.setupFunc(t)
			tm := policy.NewTrustManager(pol, policyPath)

			got, err := tm.LoadTrustedRuntimeConfig(targetPath, tc.prompter, tc.mode)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)

				return
			}

			require.NoError(t, err)

			if tc.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, "kat.jacobcolvin.com/v1beta1", got.GetAPIVersion())
				assert.Equal(t, "RuntimeConfig", got.GetKind())
			}

			if tc.checkTrustList {
				assert.True(t, pol.IsTrusted(targetPath))
			}
		})
	}
}
