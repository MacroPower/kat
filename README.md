<p align="center">
  <a href="#"><img src="docs/assets/logo.svg" width="200px"></a>
  <h1 align="center">kat</h1>
</p>

<p align="center">
  <code>kat</code> is like <code>cat</code> for projects that render Kubernetes manifests. It provides a pretty terminal UI to quickly <b>find</b>, <b>debug</b>, and <b>reload</b> manifests, without ever leaving your shell.
</p>

<p align="center">
  <code>kat</code> is designed to reduce inner loop time for <b>developers</b> and <b>platform engineers</b> working on things like <code>helm</code> charts and <code>kustomize</code> projects. By defining custom rules in the config, you can make <code>kat</code> work with anything that generates Kubernetes manifests!
</p>

<p align="center">
  <br>
  <img src="./docs/assets/demo.gif">
</p>

<p align="center">
  ❤️ Made with <a href="https://github.com/charmbracelet/bubbletea">bubble tea</a> and <a href="https://github.com/charmbracelet/glow">glow</a>.
</p>

## ✨ Features

- 🚀 List and filter hundreds of manifests without leaving your shell.
- 🔄 Reload from any context to quickly diff individual manifests.
- 🐛 Immediately view any errors from rendering, and re-reload!
- 🎨 Customize keybinds, styles, and more to match your preferences.
- 🪄 Add your own commands and rules to detect different project types.
- 🚨 Define custom hooks to automatically validate rendered manifests.

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
  -h, --help                        Show context-sensitive help.
      --ui-minimum-delay=500ms      Minimum delay for UI updates ($KAT_UI_MINIMUM_DELAY).
      --ui-glamour-style="auto"     Glamour style for rendering ($KAT_UI_GLAMOUR_STYLE).
      --ui-glamour-max-width=0      Maximum width for glamour rendering ($KAT_UI_GLAMOUR_MAX_WIDTH).
      --ui-glamour-disabled         Disable glamour rendering ($KAT_UI_GLAMOUR_DISABLED).
      --ui-line-numbers-disabled    Disable line numbers in the UI ($KAT_UI_LINE_NUMBERS_DISABLED).
      --ui-compact                  Enable compact mode for the UI ($KAT_UI_COMPACT).
      --log-level="info"            Log level ($KAT_LOG_LEVEL).
      --log-format="text"           Log format ($KAT_LOG_FORMAT).
  -f, --file=FILE                   File content to read.
      --write-config                Write the configuration file to the default path.
      --show-config                 Print the active configuration and exit.
```

## ⚙️ Configuration

You can use `kat --write-config` to generate a default configuration file at `~/.config/kat/config.yaml`. This file allows you to customize the behavior of `kat`, such as the UI style, keybindings, and commands.

Alternatively, you can use `kat --show-config` to print the active configuration and redirect the output to a file.

You can also find an example configuration file in [example/config.yaml](example/config.yaml).

### Custom Commands

You can customize the commands that `kat` runs in the configuration file. These rules match files or directories and specify the command to run when `kat` is invoked.

Additionally, you can define hooks:

- `preRender` hooks are executed before the main command is run, allowing you to prepare the environment (e.g., running `helm dependency build`).
- `postRender` hooks are executed after the main command has run, allowing you to process the rendered manifests (e.g., validating them with `kubeconform`).

If any hooks exit with a non-zero status, `kat` will display the error message. You can dismiss the error message and return to the main view, or make edits and press `r` to re-run the command.

```yaml
kube:
  commands:
    # Run `helm template . --generate-name` when kat targets a directory
    # containing a `Chart.yaml` file.
    - match: .*/Chart\.ya?ml
      command: helm
      args: [template, ".", --generate-name]
      hooks:
        preRender:
          - command: helm
            args: [dependency, build]
        postRender:
          # Pass the rendered manifests via stdin to `kubeconform`.
          - &kubeconform
            command: kubeconform
            args: [-strict, -summary]

    # Run `kustomize build --enable-helm .` when kat targets a directory
    # containing a `kustomization.yaml` file.
    - match: .*/kustomization\.ya?ml
      command: kustomize
      args: [build, --enable-helm, "."]
      hooks:
        postRender:
          - *kubeconform
```

## 🔍️ Similar Tools

- [bat](https://github.com/sharkdp/bat)
- [glow](https://github.com/charmbracelet/glow)
- [k9s](https://github.com/derailed/k9s)
- [viddy](https://github.com/sachaos/viddy)
- [soft-serve](https://github.com/charmbracelet/soft-serve)
- [wishlist](https://github.com/charmbracelet/wishlist)
