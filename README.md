<p align="center">
  <a href="#"><img src="docs/assets/logo.svg" width="200px"></a>
  <h1 align="center">kat</h1>
</p>

<p align="center">
  <code>kat</code> is like <code>cat</code> for projects that render Kubernetes manifests. It provides a pretty terminal UI to quickly view, filter, and reload manifests in your shell.
</p>

<p align="center">
  It is primarily designed to reduce inner loop time for Developers and Platform Engineers working on <code>helm</code> and <code>kustomize</code> projects, and is highly extensible via config.
</p>

<p align="center">
  <br>
  <img src="./docs/assets/demo.gif">
</p>

<p align="center">
  ❤️ Made with <a href="https://github.com/charmbracelet/bubbletea">bubble tea</a> and <a href="https://github.com/charmbracelet/glow">glow</a>.
</p>

## ✨ Features

- 🚀 Render and view hundreds of manifests without leaving your shell.
- 🔄 Reload from any context to quickly diff individual manifests.
- 🐛 Immediately view any errors from rendering, and re-reload!
- 🎨 Customize keybinds, styles, and more to match your preferences.
- 🪄 Define your own commands and rules to detect different project types.

## 📦 Installation

### Homebrew

```sh
brew tap macropower/tap
brew install kat
```

### Releases

Archives are posted in [releases](https://github.com/MacroPower/kat/releases).

## ⚡️ Usage

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
      --show-config                Print the active configuration and exit.
```

## ⚙️ Configuration

See the [default configuration file](example/config.yaml).

## 🔍️ Similar Tools

- [bat](https://github.com/sharkdp/bat)
- [glow](https://github.com/charmbracelet/glow)
- [k9s](https://github.com/derailed/k9s)
- [viddy](https://github.com/sachaos/viddy)
- [soft-serve](https://github.com/charmbracelet/soft-serve)
- [wishlist](https://github.com/charmbracelet/wishlist)
