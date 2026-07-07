// CI functions specific to the kat repository. Most quality gates are Taskfile
// targets that call local tools (go, golangci-lint, prettier) provided by
// devbox. These functions run those same tasks inside the project's devbox
// environment via the devbox toolchain, so CI reproduces exactly what
// developers run locally: local skips the container for speed, CI keeps it for
// reproducibility.
//
// The release pipeline (build.go, publish.go) instead composes the goreleaser
// toolchain directly -- including its folded-in cosign signing and syft SBOM
// helpers -- because those tools are not on the devbox PATH. It cross-compiles
// the pure-Go (CGO_ENABLED=0) binaries for linux/windows/darwin x amd64/arm64
// via GoReleaser, runs gen.sh (the goreleaser before-hook that builds the shell
// completions and man page), then builds kat's two container variants (the
// multi-stage scratch image and the alpine image) directly from the repo's
// Dockerfiles via Dagger's DockerBuild and publishes them as multi-arch
// manifests. LintActions runs the zizmor toolchain and Security runs the
// security toolchain (Trivy) for the same reason. Renovate-config validation
// stays self-contained here (a pinned renovate-config-validator in a Node
// container) because it is the one check neither devbox nor a shared toolchain
// provides.
package main

import (
	"context"

	"dagger/ci/internal/dagger"
)

const (
	goreleaserVersion = "v2.16.0" // renovate: datasource=github-releases depName=goreleaser/goreleaser

	defaultRegistry = "ghcr.io/macropower/kat"

	katCloneURL = "https://github.com/macropower/kat.git"

	katCacheNamespace = "github.com/macropower/kat/ci"

	// renovateConfig is the Renovate configuration file validated by
	// [Ci.LintRenovate], relative to the source root.
	renovateConfig = ".github/renovate.json5"

	// Docker Official Image, pulled from Docker's verified publisher
	// space on ECR Public to avoid Docker Hub pull rate limits.
	renovateImage   = "public.ecr.aws/docker/library/node:24-slim" // renovate: datasource=docker depName=public.ecr.aws/docker/library/node
	renovateVersion = "43.255.0"                                   // renovate: datasource=npm depName=renovate

	// zizmorConfig is the zizmor configuration file used by [Ci.LintActions],
	// relative to the source root.
	zizmorConfig = ".github/zizmor.yaml"

	// devboxHome is the home directory of the devbox image's non-root user,
	// under which the Go and golangci-lint caches are mounted.
	devboxHome = "/home/devbox"
	// devboxUser owns the mounted caches so the containerized tasks can
	// write to them.
	devboxUser = "devbox"
)

// Ci provides CI functions for the kat repository. Create instances with
// [New].
type Ci struct {
	// Project source directory.
	Source *dagger.Directory
	// Container image registry address (e.g. "ghcr.io/macropower/kat").
	Registry string
	// Directory containing only go.mod and go.sum, synced independently of
	// [Ci.Source] so that its content hash changes only when dependency files
	// change.
	GoMod *dagger.Directory // +private
	// Devbox toolchain instance the task-based checks run inside.
	Devbox *dagger.Devbox // +private
	// Goreleaser toolchain used to build, validate, sign, and release the
	// binaries, including its folded-in cosign signing and syft SBOM helpers
	// (see build.go, publish.go).
	Goreleaser *dagger.Goreleaser // +private
	// Scanner is the security toolchain (Trivy) backing [Ci.Security]. Named
	// Scanner rather than Security to avoid colliding with that method.
	Scanner *dagger.Security // +private
	// Zizmor is the zizmor toolchain backing [Ci.LintActions].
	Zizmor *dagger.Zizmor // +private
}

// New creates an [Ci] module with the given project source directory.
func New(
	// Project source directory. Ignore patterns (e.g. .git, dist) belong in the
	// root dagger.json customizations, not here.
	// +defaultPath="/"
	source *dagger.Directory,
	// Go module files (go.mod and go.sum only). Synced separately from source so
	// that the go mod download layer is cached independently of source code
	// changes.
	// +defaultPath="/"
	// +ignore=["*", "!go.mod", "!go.sum"]
	goMod *dagger.Directory,
	// Container image registry address.
	// +optional
	registry string,
) *Ci {
	if registry == "" {
		registry = defaultRegistry
	}
	return &Ci{
		Source:   source,
		GoMod:    goMod,
		Registry: registry,
		Devbox: dag.Devbox(dagger.DevboxOpts{
			Source:         source,
			CacheNamespace: katCacheNamespace,
		}),
		Goreleaser: dag.Goreleaser(dagger.GoreleaserOpts{
			Source:    source,
			Version:   goreleaserVersion,
			RemoteURL: katCloneURL,
		}),
		Scanner: dag.Security(dagger.SecurityOpts{
			Source:         source,
			CacheNamespace: katCacheNamespace + ":security",
		}),
		Zizmor: dag.Zizmor(dagger.ZizmorOpts{
			Source:     source,
			ConfigPath: zizmorConfig,
		}),
	}
}

// env returns the devbox environment container with the project source
// overlaid and the Go module, build, and golangci-lint caches mounted, ready
// to run `devbox run -- task <target>`. The caches persist across runs so the
// containerized tasks reuse work the way the local toolchain does.
func (m *Ci) env() *dagger.Container {
	owner := dagger.ContainerWithMountedCacheOpts{Owner: devboxUser}
	return m.Devbox.WithSource().
		WithMountedCache(devboxHome+"/go/pkg/mod", dag.CacheVolume(katCacheNamespace+":gomod"), owner).
		WithEnvVariable("GOMODCACHE", devboxHome+"/go/pkg/mod").
		WithMountedCache(devboxHome+"/.cache/go-build", dag.CacheVolume(katCacheNamespace+":gobuild"), owner).
		WithEnvVariable("GOCACHE", devboxHome+"/.cache/go-build").
		WithMountedCache(devboxHome+"/.cache/golangci-lint", dag.CacheVolume(katCacheNamespace+":golangci-lint"), owner)
}

// runTask runs a Taskfile target inside the devbox environment, failing if it
// exits non-zero.
func (m *Ci) runTask(ctx context.Context, target string) error {
	_, err := m.env().
		WithExec([]string{"devbox", "run", "--", "task", target}).
		Sync(ctx)
	return err
}
