// Integration tests for the [Ci] module. Individual tests are annotated with
// +check so `dagger check -m ci/tests` runs them all concurrently.
package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/tests/internal/dagger"

	"golang.org/x/sync/errgroup"
)

// Tests provides integration tests for the [Ci] module. Create instances with
// [New].
type Tests struct{}

// All runs all tests in parallel.
func (m *Tests) All(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return m.TestBuildDist(ctx) })
	g.Go(func() error { return m.TestBuildImageMetadata(ctx) })
	g.Go(func() error { return m.TestLintReleaserClean(ctx) })
	g.Go(func() error { return m.TestBinary(ctx) })
	g.Go(func() error { return m.TestLintActionsClean(ctx) })

	return g.Wait()
}

// TestBuildDist verifies that [Ci.Build] returns a dist directory containing
// expected entries (checksums and at least one platform archive).
//
// +check
func (m *Tests) TestBuildDist(ctx context.Context) error {
	entries, err := dag.Ci().Build().Entries(ctx)
	if err != nil {
		return fmt.Errorf("list dist entries: %w", err)
	}

	hasChecksums := false
	hasArchive := false
	for _, entry := range entries {
		if strings.Contains(entry, "checksums") {
			hasChecksums = true
		}
		if strings.Contains(entry, "linux_amd64") || strings.Contains(entry, "linux_arm64") {
			hasArchive = true
		}
	}

	if !hasChecksums {
		return fmt.Errorf("dist missing checksums file (entries: %v)", entries)
	}
	if !hasArchive {
		return fmt.Errorf("dist missing platform archive (entries: %v)", entries)
	}

	return nil
}

// TestBuildImageMetadata verifies that [Ci.BuildImages] produces both image
// variants (default scratch + alpine) for both architectures, each carrying the
// expected OCI labels and a kat entrypoint. The two variants ship different
// entrypoints (the scratch Dockerfile copies the binary to /kat; the alpine
// Dockerfile keeps it at /usr/local/bin/kat), so the entrypoint check accepts
// either as long as it ends in "kat".
//
// +check
func (m *Tests) TestBuildImageMetadata(ctx context.Context) error {
	dist := dag.Ci().Build()
	variants, err := dag.Ci().BuildImages(ctx, dagger.CiBuildImagesOpts{
		Version: "v0.0.0-test",
		Dist:    dist,
	})
	if err != nil {
		return fmt.Errorf("build images: %w", err)
	}
	// 2 variants (default, alpine) x 2 platforms (amd64, arm64).
	if len(variants) != 4 {
		return fmt.Errorf("expected 4 image containers (2 variants x 2 platforms), got %d", len(variants))
	}

	for i, ctr := range variants {
		version, err := ctr.Label(ctx, "org.opencontainers.image.version")
		if err != nil {
			return fmt.Errorf("variant %d: version label: %w", i, err)
		}
		if version != "v0.0.0-test" {
			return fmt.Errorf("variant %d: version label = %q, want %q", i, version, "v0.0.0-test")
		}

		title, err := ctr.Label(ctx, "org.opencontainers.image.title")
		if err != nil {
			return fmt.Errorf("variant %d: title label: %w", i, err)
		}
		if title != "kat" {
			return fmt.Errorf("variant %d: title label = %q, want %q", i, title, "kat")
		}

		created, err := ctr.Label(ctx, "org.opencontainers.image.created")
		if err != nil {
			return fmt.Errorf("variant %d: created label: %w", i, err)
		}
		if created == "" {
			return fmt.Errorf("variant %d: created label is empty", i)
		}

		ep, err := ctr.Entrypoint(ctx)
		if err != nil {
			return fmt.Errorf("variant %d: entrypoint: %w", i, err)
		}
		if len(ep) != 1 || !strings.HasSuffix(ep[0], "kat") {
			return fmt.Errorf("variant %d: entrypoint = %v, want a single kat entry", i, ep)
		}
	}

	return nil
}

// TestLintReleaserClean verifies that the GoReleaser configuration passes
// validation. This exercises the [Ci.LintReleaser] check, which requires the
// kat git remote for homebrew/nix repository resolution.
//
// +check
func (m *Tests) TestLintReleaserClean(ctx context.Context) error {
	return dag.Ci().LintReleaser(ctx)
}

// TestBinary verifies that [Ci.Binary] compiles the kat binary.
//
// +check
func (m *Tests) TestBinary(ctx context.Context) error {
	size, err := dag.Ci().Binary().Size(ctx)
	if err != nil {
		return fmt.Errorf("binary: %w", err)
	}
	if size == 0 {
		return fmt.Errorf("binary has zero size")
	}
	return nil
}

// TestLintActionsClean verifies that the GitHub Actions workflows pass zizmor
// linting. This exercises the [Ci.LintActions] check and catches workflow
// security or syntax issues.
//
// +check
func (m *Tests) TestLintActionsClean(ctx context.Context) error {
	return dag.Ci().LintActions(ctx)
}
