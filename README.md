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
- 👀 Use `--watch` to trigger reloads on changes to source files.
- 🐛 Immediately view any errors from rendering, and re-reload!
- 🎨 Customize keybinds, styles, and more to match your preferences.
- 🪄 Add your own commands and rules to detect different project types.
- 🚨 Define custom hooks to automatically validate rendered manifests.

## 📦 Installation

### Homebrew

```sh
brew tap macropower/tap
brew install kat --cask
```

### Releases

Archives are posted in [releases](https://github.com/MacroPower/kat/releases).

## ⚡️ Usage

Show help:

```sh
kat --help
```

Render a project in the current directory:

```sh
kat
```

Render a project and enable watch (live reloading):

```sh
kat -w
```

Render a project in a specific directory:

```sh
kat ./example/helm
```

Render a project in a specific directory with command passthrough:

```sh
kat ./example/helm -- helm template my-chart .
```

Render using data from stdin:

```sh
cat ./example/kustomize/resources.yaml | kat -f -
```

## ⚙️ Configuration

You can use `kat --write-config` to generate a default configuration file at `~/.config/kat/config.yaml`. This file allows you to customize the behavior of `kat`, such as the UI style, keybindings, and commands.

Alternatively, you can use `kat --show-config` to print the active configuration and redirect the output to a file.

You can also find an example configuration file in [example/config.yaml](example/config.yaml).

## 🌈 Themes

![Themes](./docs/assets/themes.gif)

Configure a theme with `--ui-theme`, `KAT_UI_THEME`, or via config:

```yaml
ui:
  theme: "dracula"
```

We use [Chroma](https://github.com/alecthomas/chroma/) for theming, so you can use any styles from the [Chroma Style Gallery](https://xyproto.github.io/splash/docs/).

You can also add your own themes in the config:

```yaml
ui:
  theme: "my-custom-theme"
  themes:
    my-custom-theme:
      styles:
        background: "#abb2bf bg:#282c34"
        punctuation: "#abb2bf"
        keyword: "#c678dd"
        name: "bold #e06c75"
        comment: "italic #8b949e"
        commentSpecial: "bold italic #8b949e"
        # ...
```

Chroma uses the same syntax as Pygments. Define `ui.themes.[name].styles` as a map of Pygments [Tokens](https://pygments.org/docs/tokens/) to [Styles](http://pygments.org/docs/styles/). You can then reference any theme in `ui.theme` (or by using the corresponding flag / env var).

## 🛠️ Commands

You can customize the commands that `kat` runs in the configuration file. This allows you to define how `kat` should render different types of projects. For example, you can define specific commands and arguments for rendering `helm` charts or `kustomize` projects, or add support for other languages/tools like [`kcl`](https://www.kcl-lang.io/) (maybe with [`kclipper`](https://github.com/MacroPower/kclipper)?), [`jsonnet`](https://jsonnet.org/), [`flux-local`](https://github.com/allenporter/flux-local), [`cue`](https://cuelang.org/), and so on.

Commands are defined as an ordered list under `kube.commands`, and each command has three required properties: `match`, `command`, and `args`.

```yaml
kube:
  commands:
    - match: .*/Chart\.ya?ml$
      command: helm
      args: [template, ".", --generate-name]
```

When `kat` is invoked, it will check each command in the order they are defined. The `match` property is a regular expression that matches the files in the provided directory path. If a match is found, `kat` will run the specified `command` with the provided `args`.

In the above example, running `kat example/helm` will evaluate `match` against the files in the `example/helm` directory. If a file matches the regex (e.g., `Chart.yaml`), it will run the command `helm template . --generate-name`.

Additionally, you can optionally specify a `source` regex to filter files that should trigger the command when using `-w, --watch`. Note that the entire file tree is walked, so the regex will be evaluated against all files in the directory and its subdirectories.

```yaml
kube:
  commands:
    - match: .*/Chart\.ya?ml$
      source: .*\.(ya?ml|tpl)$ # Reload on any changes to YAML or template files.
      command: helm
      args: [template, ".", --generate-name]
```

> Eventually, more sophisticated rules for match/source will be supported, but this method is simple and effective for many use cases.

### 🪝 Hooks

You can optionally define hooks for your commands:

- `init` hooks are executed once when `kat` is initialized, allowing you to run any one-time initialization tasks (e.g., checking that `helm` is available).
- `preRender` hooks are executed before the main command is run, allowing you to prepare the environment (e.g., running `helm dependency build`).
- `postRender` hooks are executed after the main command has run and are provided the rendered output via stdin, allowing you to process the rendered manifests (e.g., validating them with `kubeconform`).

Like `commands`, hooks are defined as an ordered list of commands under the respective hook type. Each hook command has its own `command` and `args`.

If any hooks exit with a non-zero status, `kat` will display the error message. You can dismiss the error message and return to the main view, or trigger a reload to re-render the manifests and re-run the render hooks.

```yaml
kube:
  commands:
    - match: .*/Chart\.ya?ml$
      source: .*\.(ya?ml|tpl)$
      command: helm
      args: [template, ".", --generate-name]
      hooks:
        postRender:
          # Pass the rendered manifests via stdin to `kubeconform`.
          - command: kubeconform
            args: [-strict, -summary]
```

### 📖 Examples

By default, `kat` includes a minimal configuration that supports `helm` and `kustomize`. This is a great starting point for your own custom config.

```yaml
kube:
  commands:
    - match: .*/Chart\.ya?ml$
      source: .*\.(ya?ml|tpl)$
      command: helm
      args: [template, ".", --generate-name]
      hooks:
        init:
          - command: helm
            args: [version, --short]
        preRender:
          - command: helm
            args: [dependency, build]
    - match: .*/kustomization\.ya?ml$
      source: .*\.ya?ml$
      command: kustomize
      args: [build, "."]
      hooks:
        init:
          - command: kustomize
            args: [version]
```

If you use something like `make` or [`task`](https://taskfile.dev), you can alternatively use these in the `kat` config. Additionally, this allows you to swap out backend rendering implementations very easily.

```yaml
kube:
  commands:
    - match: .*/Taskfile\.ya?ml$
      command: task
      args: [render]
      hooks:
        postRender:
          - command: task
            args: [validate]
```

## 🔍️ Similar Tools

- [bat](https://github.com/sharkdp/bat)
- [glow](https://github.com/charmbracelet/glow)
- [k9s](https://github.com/derailed/k9s)
- [viddy](https://github.com/sachaos/viddy)
- [soft-serve](https://github.com/charmbracelet/soft-serve)
- [wishlist](https://github.com/charmbracelet/wishlist)
