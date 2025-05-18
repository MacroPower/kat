<p align="center">
  <a href="#"><img src="docs/assets/logo.svg" width="250px"></a>
  <h1 align="center">kat</h1>
</p>

<p align="center"><i><code>cat</code> for Kubernetes manifests</i></p>

## Overview

- Easily filter and view hundreds of manifests in your shell
- Quickly iterate over changes to a single manifest (press `r` to reload)
- Automatically detect and render `helm` and `kustomize` projects
- Can be extended via `$XDG_CONFIG_HOME/kat/config.yaml`

![demo](docs/assets/demo.gif)

Uses [bubbletea](https://github.com/charmbracelet/bubbletea) and code from [glow](https://github.com/charmbracelet/glow).

## Usage

```sh
# kat the current directory.
kat .

# kat a file or directory path.
kat ./example/kustomize

# kat with command passthrough.
kat ./example/helm -- helm template foobar .
```

## Installation

### Homebrew

```sh
brew tap macropower/tap
brew install kat
```

### Releases

Archives are posted in [releases](https://github.com/MacroPower/kat/releases).

## TODO

- Support manifests from stdin.
- Watch/reload (i.e. re-run commands automatically).
- Vim keybindings inside the YAML viewer.

## Similar Tools

- [bat](https://github.com/sharkdp/bat)
- [glow](https://github.com/charmbracelet/glow)
- [k9s](https://github.com/derailed/k9s)
- [viddy](https://github.com/sachaos/viddy)
