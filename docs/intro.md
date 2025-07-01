# Introducing kat: A TUI for rendering and validating manifests

Hello! I'm excited to introduce `kat`, an open-source, TUI application I built in my free time to improve my local development workflow.



## Features

**Manifest Browsing**: Rather than outputing a single long stream of YAML, `kat` organizes the output into a browsable list structure. Navigate through any number of rendered manifests using their group/kind/ns/name metadata.

**Live Reload**: Just use the `-w` flag to automatically re-render when you modify source files, without losing your current position or context when the output changes.

**Integrated Validation**: Run tools like `kubeconform`, `kyverno`, or custom validators automatically on rendered output through configurable hooks. Additionally, you can define custom "plugins", which function the same way as k9s plugins (i.e. commands invoked with a keybind).

**Flexible Configuration**: `kat` allows you to define profiles for different manifest generators (like Helm, Kustomize, etc.). Profiles can be automatically selected based on output of CEL expressions, allowing `kat` to adapt to your project structure. For example, you can express rules like:

  - If there is a `kustomization.yaml` file in the current directory, use the Kustomize profile to render manifests.
  - If that Kustomization also contains some Fluxtomizations, use [flux-local](https://github.com/allenporter/flux-local) instead.
  - Call `task flux:validate` to run a custom validation task after rendering manifests.

If you're interested in giving it a try, there are installation and usage instructions available in the repo's README:

[github.com/macropower/kat](https://github.com/macropower/kat)

## But why?

For me, I found that working with manifest generators often involved a repetitive cycle:

1. Run `helm template`, `kustomize build`, or similar commands
2. Search through many pages of output looking for specific resources
3. Find some issue and make a change to the source files
4. Re-run the rendering commands
5. Re-run whatever search I originally did
6. Find another issue and make a change to the source files
7. Repeat ad nauseam

While this was something I had tolerated for a while, it finally came to a head when I was debugging one particularly bad helm chart, and finally decided that I had to rework my workflow in some way. After not being able to find any solutions that I liked, I ended up building `kat`.

## What is `kat`?



## Closing Thoughts

`kat` solved my specific workflow problems when working with Kubernetes manifests locally. And while it may not be a perfect fit for everyone, I hope it can help others who find themselves in a similar situation.
