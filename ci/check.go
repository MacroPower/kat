package main

import (
	"context"

	"dagger/ci/internal/dagger"
)

// Lint runs the lint gate (golangci-lint, go mod tidy check, prettier) inside
// the devbox environment, mirroring `task lint`.
//
// +check
func (m *Ci) Lint(ctx context.Context) error {
	return m.runTask(ctx, "lint")
}

// Test runs the unit tests with the race detector inside the devbox
// environment, mirroring `task go:test`.
//
// +check
func (m *Ci) Test(ctx context.Context) error {
	return m.runTask(ctx, "go:test")
}

// TestCoverage runs all tests with coverage profiling inside the devbox
// environment (mirroring `task go:test:cover`) and returns the coverage profile
// file.
func (m *Ci) TestCoverage() *dagger.File {
	return m.env().
		WithExec([]string{"devbox", "run", "--", "task", "go:test:cover"}).
		File(".test/coverage.txt")
}

// LintReleaser validates the GoReleaser configuration. Delegates to the
// shared [Goreleaser] toolchain, which mounts the source over a minimal git
// repo (the kat remote URL is configured at construction) because the
// goreleaser config references a git remote for homebrew/nix repository
// resolution.
//
// +check
func (m *Ci) LintReleaser(ctx context.Context) error {
	return m.Goreleaser.Check(ctx)
}

// LintActions lints the GitHub Actions workflows for security issues by
// composing the zizmor toolchain directly. zizmor is not on the devbox PATH, so
// this gate does not run through devbox. It pins .github/zizmor.yaml as the
// config path rather than relying on zizmor's auto-discovery.
//
// +check
func (m *Ci) LintActions(ctx context.Context) error {
	return m.Zizmor.Lint(ctx)
}

// Security scans source dependencies for known vulnerabilities by composing the
// security toolchain (Trivy) directly. The scanned source is the `ci`
// toolchain's source, whose root dagger.json customization already excludes the
// build and cache directories.
//
// +check
func (m *Ci) Security(ctx context.Context) error {
	return m.Scanner.ScanSource(ctx)
}

// SecuritySourceSarif scans source dependencies for known vulnerabilities and
// returns the results as a SARIF file for upload to GitHub Code Scanning. Unlike
// [Ci.Security], it does not gate on findings: SARIF capture must produce the
// file even when vulnerabilities are present, so they can be published to the
// Security tab. It scans the same source as the gate.
func (m *Ci) SecuritySourceSarif() *dagger.File {
	return m.Scanner.ScanSourceSarif()
}

// SecurityImageSarif builds a runtime image and scans it for known
// vulnerabilities, returning the results as a SARIF file for upload to GitHub
// Code Scanning. It composes the release image builder ([Ci.BuildImages]) so it
// scans exactly what a release publishes, then scans the native linux/amd64
// variant (Dagger evaluates only that variant lazily). It scans the alpine
// variant, which carries the OS-layer packages (curl, git, helm, kustomize, yq)
// the scratch image lacks, so it surfaces OS-layer CVEs the source scan, seeing
// only Go modules, cannot. Unlike the gating scans it does not fail on findings.
func (m *Ci) SecurityImageSarif(
	ctx context.Context,
	// Version label for OCI metadata on the scanned image.
	// +default="0.0.0-scan"
	version string,
) (*dagger.File, error) {
	variants, err := m.BuildImages(ctx, version, nil, string(VariantAlpine))
	if err != nil {
		return nil, err
	}

	return m.Scanner.ScanImageSarif(variants[0]), nil
}

// LintRenovate validates the Renovate configuration with
// renovate-config-validator, installed at a pinned version in a Node container
// so the check is self-contained and Renovate can bump its own validator
// version. It is the one check that composes neither devbox nor a shared
// toolchain.
//
// +check
func (m *Ci) LintRenovate(ctx context.Context) error {
	_, err := dag.Container().
		From(renovateImage).
		WithMountedCache("/root/.npm", dag.CacheVolume(katCacheNamespace+":npm")).
		WithExec([]string{"npm", "install", "-g", "renovate@" + renovateVersion}).
		WithMountedFile("/src/"+renovateConfig, m.Source.File(renovateConfig)).
		WithWorkdir("/src").
		WithExec([]string{"renovate-config-validator", renovateConfig}).
		Sync(ctx)
	return err
}
