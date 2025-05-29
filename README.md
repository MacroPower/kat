<p align="center">
  <a href="#"><img src="docs/assets/logo.svg" width="250px"></a>
  <h1 align="center">kat</h1>
</p>

<p align="center"><i><code>cat</code> for Kubernetes manifests</i></p>

## Overview

- Easily filter and view hundreds of manifests in your shell
- Quickly iterate over changes to a single manifest (press `r` to reload)
- Automatically detect and render `helm` and `kustomize` projects
- Define your own rules and keybinds in `~/.config/kat/config.yaml`

![demo](docs/assets/demo.gif)

Uses [bubbletea](https://github.com/charmbracelet/bubbletea) and code from [glow](https://github.com/charmbracelet/glow).

## Usage

```console
$ kat --help

Usage: kat [<path> [<command> ...]] [flags]

cat for Kubernetes manifests.

Examples:

    # kat the current directory.
    kat .

    # kat a file or directory path.
    kat ./example/kustomize

    # kat with command passthrough.
    kat ./example/kustomize -- kustomize build .

    # kat a file or stdin directly (no reload support).
    cat ./example/kustomize/resources.yaml | kat -f -

Arguments:
  [<path>]           File or directory path, default is $PWD.
  [<command> ...]    Command to run, defaults set in ~/.config/kat/config.yaml.

Flags:
  -h, --help                       Show context-sensitive help.
      --log-level="info"           Log level ($KAT_LOG_LEVEL).
      --log-format="text"          Log format ($KAT_LOG_FORMAT).
  -f, --file=FILE                  File content to read.
      --ui-glamour-style=STRING    ($KAT_UI_GLAMOUR_STYLE)
      --ui-glamour-max-width=INT   ($KAT_UI_GLAMOUR_MAX_WIDTH)
      --ui-glamour-disabled        ($KAT_UI_GLAMOUR_DISABLED)
      --ui-show-line-numbers       ($KAT_UI_SHOW_LINE_NUMBERS)
      --ui-enable-mouse            ($KAT_UI_ENABLE_MOUSE)
```

Also see the [default configuration file](example/config.yaml).

## Installation

### Homebrew

```sh
brew tap macropower/tap
brew install kat
```

### Releases

Archives are posted in [releases](https://github.com/MacroPower/kat/releases).

## Similar Tools

- [bat](https://github.com/sharkdp/bat)
- [glow](https://github.com/charmbracelet/glow)
- [k9s](https://github.com/derailed/k9s)
- [viddy](https://github.com/sachaos/viddy)
- [soft-serve](https://github.com/charmbracelet/soft-serve)
- [wishlist](https://github.com/charmbracelet/wishlist)
