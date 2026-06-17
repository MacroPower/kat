package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/ci/internal/dagger"
)

// Variant identifies a container image variant. Each variant uses a different
// Dockerfile. See [VariantDefault] and [VariantAlpine] for the available
// variants.
type Variant string

const (
	// VariantDefault builds the multi-stage scratch image from docker/Dockerfile.
	// It is the default variant and produces the smallest image: an alpine build
	// stage that exercises the binary, then a scratch final stage holding only
	// the kat binary and its default config.
	VariantDefault Variant = "default"

	// VariantAlpine builds the alpine image from docker/Dockerfile.alpine, with
	// helm, kustomize, yq, the shell completions, and the man page installed.
	VariantAlpine Variant = "alpine"
)

// allVariants lists every supported image variant in publishing order.
var allVariants = []Variant{VariantDefault, VariantAlpine}

// dockerfile maps a variant to its Dockerfile path under the source root.
func (v Variant) dockerfile() string {
	if v == VariantAlpine {
		return "docker/Dockerfile.alpine"
	}
	return "docker/Dockerfile"
}

// variantSet groups multi-arch platform containers for a single image variant.
// Used internally by [buildAllImages] to keep variant metadata associated with
// its containers through the publish pipeline.
type variantSet struct {
	variant    Variant
	containers []*dagger.Container
}

// platforms is the set of architectures every image variant is built for.
var platforms = []dagger.Platform{"linux/amd64", "linux/arm64"}

// distBinaryDir maps a linux platform to its GoReleaser dist directory. kat is
// built CGO_ENABLED=0, so amd64 lands under the v1 microarchitecture and arm64
// under v8.0, matching GoReleaser's default naming.
func distBinaryDir(platform dagger.Platform) string {
	if platform == "linux/arm64" {
		return "kat_linux_arm64_v8.0"
	}
	return "kat_linux_amd64_v1"
}

// Build runs GoReleaser in snapshot mode, producing binaries for all
// platforms. Returns the dist/ directory. Source archives are skipped in
// snapshot mode since they are only needed for releases. Docker is skipped
// because images are built natively via Dagger (see [Ci.BuildImages]).
func (m *Ci) Build(ctx context.Context) (*dagger.Directory, error) {
	ctr, err := m.releaserBase(ctx)
	if err != nil {
		return nil, err
	}
	return ctr.
		WithExec([]string{
			"goreleaser", "release", "--snapshot", "--clean",
			"--skip=docker,homebrew,nix,sign,sbom",
			"--parallelism=0",
		}).
		Directory("/src/dist"), nil
}

// Binary compiles the kat binary for the given platform. kat is CGO-free, so
// CGO_ENABLED=0 produces a static binary. Used by the tests and the local
// `install` task. The go module and build caches are keyed on this module's
// cache namespace and persist across runs; GoMod is mounted before source so
// the module download layer caches independently of source changes.
func (m *Ci) Binary(
	// Target build platform (e.g. "darwin/arm64"). Defaults to the engine's
	// native platform.
	// +optional
	platform dagger.Platform,
) *dagger.File {
	ctr := m.Goreleaser.GoreleaserBase().
		WithMountedCache("/go/pkg/mod", dag.CacheVolume(katCacheNamespace+":gomod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume(katCacheNamespace+":gobuild")).
		WithEnvVariable("GOCACHE", "/root/.cache/go-build").
		WithWorkdir("/src").
		WithMountedDirectory("/src", m.GoMod).
		WithExec([]string{"go", "mod", "download"}).
		WithMountedDirectory("/src", m.Source).
		WithEnvVariable("CGO_ENABLED", "0")

	if platform != "" {
		goos, goarch, _ := strings.Cut(string(platform), "/")
		ctr = ctr.
			WithEnvVariable("GOOS", goos).
			WithEnvVariable("GOARCH", goarch)
	}

	return ctr.
		WithExec([]string{
			"go", "build", "-trimpath", "-ldflags", "-s -w",
			"-o", "/out/kat", "./cmd/kat",
		}).
		File("/out/kat")
}

// genArtifacts returns the directory holding the shell completions
// (completion/) and man page (man/) produced by gen.sh. The alpine image needs
// them in its build context. gen.sh is the goreleaser before-hook; here it is
// invoked directly on the release base so the artifacts can be produced without
// a full release run.
func (m *Ci) genArtifacts(ctx context.Context) (*dagger.Directory, error) {
	ctr, err := m.releaserBase(ctx)
	if err != nil {
		return nil, err
	}
	return ctr.
		WithExec([]string{"bash", "gen.sh"}).
		Directory("/src"), nil
}

// BuildImages builds multi-arch runtime container images from a GoReleaser
// dist directory. If no dist is provided, a snapshot build is run. When variant
// is empty, all variants are built and returned as a flat slice.
func (m *Ci) BuildImages(
	ctx context.Context,
	// Version label for OCI metadata.
	// +default="snapshot"
	version string,
	// Pre-built GoReleaser dist directory. If not provided, runs a snapshot build.
	// +optional
	dist *dagger.Directory,
	// Image variant to build. One of "default", "alpine". When empty, all
	// variants are built.
	// +optional
	variant string,
) ([]*dagger.Container, error) {
	if dist == nil {
		var err error
		dist, err = m.Build(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Both the source files (docker/, gen.sh artifacts) and the cross-compiled
	// binaries are needed to assemble each build context.
	gen, err := m.genArtifacts(ctx)
	if err != nil {
		return nil, err
	}

	if variant != "" {
		return buildVariantImages(dist, gen, version, Variant(variant), time.Now().UTC().Format(time.RFC3339)), nil
	}

	sets := buildAllImages(dist, gen, version)
	var all []*dagger.Container
	for _, s := range sets {
		all = append(all, s.containers...)
	}
	return all, nil
}

// buildAllImages builds multi-arch containers for every supported variant,
// returning them grouped so callers can apply variant-specific tags.
func buildAllImages(dist, gen *dagger.Directory, version string) []variantSet {
	created := time.Now().UTC().Format(time.RFC3339)
	sets := make([]variantSet, len(allVariants))
	for i, v := range allVariants {
		sets[i] = variantSet{variant: v, containers: buildVariantImages(dist, gen, version, v, created)}
	}
	return sets
}

// buildVariantImages constructs multi-arch containers for a single variant,
// sharing the created timestamp across all platforms for consistency.
func buildVariantImages(dist, gen *dagger.Directory, version string, variant Variant, created string) []*dagger.Container {
	containers := make([]*dagger.Container, len(platforms))
	for i, platform := range platforms {
		containers[i] = buildVariantImage(dist, gen, version, variant, platform, created)
	}
	return containers
}

// buildVariantImage assembles a build context for a single (variant, platform)
// and runs the variant's Dockerfile through Dagger's DockerBuild.
//
// The context root holds the cross-compiled `kat` binary (from dist) and the
// `docker/` directory; the alpine variant additionally needs `completion/` and
// `man/` (its Dockerfile COPYs them), so those are added from the gen.sh
// artifacts. TARGETOS/TARGETARCH are passed as build args, matching the
// ARG declarations in both Dockerfiles, and the platform is fixed so the alpine
// base image and the binary architecture agree. The OCI labels mirror the
// build_flag_templates in .goreleaser.yaml.
func buildVariantImage(dist, gen *dagger.Directory, version string, variant Variant, platform dagger.Platform, created string) *dagger.Container {
	_, goarch, _ := strings.Cut(string(platform), "/")

	bctx := dag.Directory().
		WithFile("kat", dist.File(distBinaryDir(platform)+"/kat")).
		WithDirectory("docker", gen.Directory("docker"))

	if variant == VariantAlpine {
		bctx = bctx.
			WithDirectory("completion", gen.Directory("completion")).
			WithDirectory("man", gen.Directory("man"))
	}

	return bctx.
		DockerBuild(dagger.DirectoryDockerBuildOpts{
			Dockerfile: variant.dockerfile(),
			Platform:   platform,
			BuildArgs: []dagger.BuildArg{
				{Name: "TARGETOS", Value: "linux"},
				{Name: "TARGETARCH", Value: goarch},
			},
		}).
		// OCI labels from .goreleaser.yaml build_flag_templates.
		WithLabel("org.opencontainers.image.title", "kat").
		WithLabel("org.opencontainers.image.version", version).
		WithLabel("org.opencontainers.image.source", katSourceURL).
		WithLabel("org.opencontainers.image.created", created).
		WithAnnotation("org.opencontainers.image.title", "kat").
		WithAnnotation("org.opencontainers.image.version", version).
		WithAnnotation("org.opencontainers.image.source", katSourceURL).
		WithAnnotation("org.opencontainers.image.created", created)
}

// katSourceURL is the OCI source label value, matching the GitURL GoReleaser
// emits for the docker images.
const katSourceURL = "https://github.com/macropower/kat"

// releaserBase builds the release container: the shared GoReleaser base (the Go
// build base plus the goreleaser binary, from the [Goreleaser] toolchain)
// extended with cosign and syft (GoReleaser's sign and sbom steps invoke them),
// the project source mounted at /src, and a bootstrapped git repo. gen.sh runs
// as GoReleaser's before-hook inside this container (it needs bash and go, both
// present in the golang base image). Config-only validation goes through the
// [Goreleaser] toolchain directly -- see [Ci.LintReleaser].
func (m *Ci) releaserBase(_ context.Context) (*dagger.Container, error) {
	// WithCosign/WithSyft take and return a container, so they are applied as
	// statements rather than chained. Install them before committing source so
	// that source changes only invalidate layers from EnsureGitRepo onward.
	ctr := m.Goreleaser.GoreleaserBase()
	ctr = m.Goreleaser.WithCosign(ctr)
	ctr = m.Goreleaser.WithSyft(ctr)
	ctr = ctr.
		// Env vars used by GoReleaser ldflags and templates (BuildUser).
		WithEnvVariable("HOSTNAME", "dagger").
		WithEnvVariable("USER", "dagger").
		// Mount source after all tools so that source changes only invalidate
		// layers from here onward, preserving the tool installation layers above.
		WithMountedDirectory("/src", m.Source)
	return m.Goreleaser.EnsureGitRepo(ctr, dagger.GoreleaserEnsureGitRepoOpts{
		RemoteURL: katCloneURL,
	}), nil
}

// ReleaseDryRun validates the image pipeline without publishing. It builds
// snapshot binaries via GoReleaser, then constructs every container image
// variant for every platform and syncs them, catching cross-compilation
// failures, missing context files, and Dockerfile errors that would surface
// only during a real release.
//
// For a fast goreleaser config-only check, see [Ci.LintReleaser].
func (m *Ci) ReleaseDryRun(ctx context.Context) error {
	dist, err := m.Build(ctx)
	if err != nil {
		return err
	}
	containers, err := m.BuildImages(ctx, "dry-run", dist, "")
	if err != nil {
		return err
	}
	for i, ctr := range containers {
		if _, err := ctr.Sync(ctx); err != nil {
			return fmt.Errorf("sync image %d: %w", i, err)
		}
	}
	return nil
}
