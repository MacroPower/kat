<p align="center">
  <a href="#"><img src="docs/assets/logo.svg" width="200px"></a>
  <h1 align="center">kat</h1>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/macropower/kat"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/macropower/kat.svg"></a>
  <a href="https://goreportcard.com/report/github.com/macropower/kat"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/macropower/kat"></a>
  <a href="https://codecov.io/gh/macropower/kat"><img src="https://codecov.io/gh/macropower/kat/graph/badge.svg?token=4TNYTL2WXV"/></a>
  <a href="#-installation"><img alt="GitHub Downloads" src="https://img.shields.io/github/downloads/macropower/kat/total"></a>
  <a href="#-installation"><img alt="Latest tag" src="https://img.shields.io/github/v/tag/macropower/kat?label=version&sort=semver"></a>
  <a href="https://github.com/macropower/kat/blob/main/LICENSE"><img alt="License" src="https://img.shields.io/github/license/macropower/kat"></a>
</p>

<p align="center">
  <code>kat</code> provides a <b>terminal UI</b> for rendering, validating, and displaying <b>local Kubernetes manifests</b>. It eliminates the frustrating cycle of manually running commands, scrolling through endless output, and constantly losing context when working with Helm charts, Kustomize overlays, and other manifest generators.
</p>

<p align="center">
  <br>
  <img src="./docs/assets/demo.gif">
</p>

<p align="center">
  ❤️ Made with <a href="https://github.com/charmbracelet/bubbletea">bubble tea</a>, <a href="https://github.com/charmbracelet/glow">glow</a>, and <a href="https://github.com/alecthomas/chroma/">chroma</a>.
</p>

## ✨ Features

- **🔍️ Manifest browsing** - Navigate hundreds of rendered manifests with fuzzy search and filtering, no more endless scrolling through terminal output
- **⚡️ Live reload** - Use `--watch` to automatically re-render when you modify source files, without losing your current context
- **🐛 Error handling** - Rendering and validation errors are displayed as overlays and disappear if reloading resolves the error
- **🎯 Project detection** - Automatically detect Helm charts, Kustomize projects, and custom manifest generators using powerful CEL expressions
- **🧪 Tool integration** - Define profiles for any manifest generator (Helm, Kustomize, CUE, KCL, Jsonnet, etc.) with pre/post-render hooks
- **🔌 Plugin system** - Create custom keybind-triggered commands for common tasks that can't run on hooks, like dry-runs or diffs
- **✅ Custom validation** - Run tools like `kubeconform`, `kyverno`, or custom validators automatically on rendered output
- **🎨 Beautiful UI** - Syntax-highlighted YAML with customizable themes and keybindings that match your preferences

## 📦 Installation

### Homebrew

```sh
brew install macropower/tap/kat --cask
```

### Go

```sh
go install github.com/macropower/kat/cmd/kat@latest
```

### Docker

Run the latest image:

```sh
docker run -it ghcr.io/macropower/kat:latest
```

Note that this image does _not_ contain any additional tools (e.g. helm, kustomize). You must create your own image to include any tools you need, for example:

```dockerfile
FROM alpine:latest
COPY --from=ghcr.io/macropower/kat:latest /kat /usr/local/bin/kat
RUN apk add --no-cache helm kustomize kubeconform
```

### Nix

You can install `kat` using my [NUR](https://github.com/MacroPower/nur-packages).

With `nix-env`:

```sh
nix-env -iA kat -f https://github.com/macropower/nur-packages/archive/main.tar.gz
```

With `nix-shell`:

```sh
nix-shell -A kat https://github.com/macropower/nur-packages/archive/main.tar.gz
```

With your `flake.nix`:

```nix
{
  inputs = {
    macropower.url = "github:macropower/nur-packages";
  };
  # Reference the package as `inputs.macropower.packages.<system>.kat`
}
```

With [`devbox`](https://www.jetify.com/docs/devbox/):

```sh
devbox add github:macropower/nur-packages#kat
```

### GitHub CLI

```sh
gh release download -R macropower/kat -p "kat_$(uname -s)_$(uname -m).tar.gz" -O - | tar -xz
```

And then move `kat` to a directory in your `PATH`.

### Curl

```sh
curl -s https://api.github.com/repos/macropower/kat/releases/latest | \
  jq -r ".assets[] |
    select(.name | test(\"kat_$(uname -s)_$(uname -m).tar.gz\")) |
    .browser_download_url" | \
  xargs curl -L | tar -xz
```

And then move `kat` to a directory in your `PATH`.

### Manual

You can download binaries from [releases](https://github.com/macropower/kat/releases).

## 🔏 Verification

You can verify the authenticity and integrity of `kat` releases.

See [verification](docs/verification.md) for more details.

## 🚀 Usage

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

Render a project in a specific directory using the `ks` profile:

```sh
kat ./example/kustomize ks
```

Render a project and override the profile arguments:

```sh
kat ./example/kustomize ks -- build . --enable-helm
```

Render a project with command passthrough:

```sh
kat ./example/helm task -- helm:render
```

Render using data from stdin:

```sh
cat ./example/kustomize/resources.yaml | kat -f -
```

## ⚙️ Configuration

You can use `kat --write-config` to generate a default configuration file at `~/.config/kat/config.yaml`. This file allows you to customize the behavior of `kat`, such as the UI style, keybindings, rules for project detection, and profiles for rendering different types of projects.

Alternatively, you can find the default configuration file in [pkg/config/config.yaml](pkg/config/config.yaml).

## 🛠️ Rules and Profiles

You can customize how `kat` detects and renders different types of projects using **rules** and **profiles** in the configuration file. This system uses [CEL (Common Expression Language)](https://cel.dev/) expressions to provide flexible file matching and processing.

### 🎯 Rules

**Rules determine which profile should be used.** Each rule contains:

- `match` (required): A CEL expression that returns `true` if the rule should be applied
- `profile` (required): The name of the profile to use when this rule matches

Rules use boolean CEL expressions with access to:

- `files` (list<string>): All file paths in the directory
- `dir` (string): The directory path being processed

```yaml
rules:
  - # Select the Helm profile if any Helm chart files exist
    match: >-
      files.exists(f, pathBase(f) in ["Chart.yaml", "Chart.yml"])
    profile: helm

  - # Select the Kustomize profile if any Kustomization files exist
    match: >-
      files.exists(f, pathBase(f) in ["kustomization.yaml", "kustomization.yml"])
    profile: ks

  - # Fallback: select the YAML profile if any YAML files exist
    match: >-
      files.exists(f, pathExt(f) in [".yaml", ".yml"])
    profile: yaml
```

### 🎭 Profiles

**Profiles define how to render projects.** They can be automatically selected by rules, or manually specified when `kat` is invoked. Each profile contains:

- `command` (required): The command to execute
- `args`: Arguments to pass to the command
- `source`: Define which files to watch for changes (when watch is enabled)
- `ui`: UI configuration overrides
- `hooks`: Initialization and rendering hooks
  - `init` hooks are executed once when `kat` is initialized
  - `preRender` hooks are executed before the profile's command is run
  - `postRender` hooks are executed after the profile's command has run, and are provided the rendered output via stdin
- `plugins`: Custom commands that can be executed on-demand with keybinds
  - `description` (required): Human-readable description of what the plugin does
  - `keys` (required): Array of key bindings that trigger the plugin
  - `command` (required): The command to execute
  - `args`: Arguments to pass to the command

Profile `source` expressions use list-returning CEL expressions with the same variables as rules.

```yaml
profiles:
  helm:
    command: helm
    args: [template, ., --generate-name]
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])
    ui:
      theme: dracula
    hooks:
      init:
        - command: helm
          args: [version, --short]
      preRender:
        - command: helm
          args: [dependency, build]
      postRender:
        # Pass the rendered manifests via stdin to `kubeconform`.
        - command: kubeconform
          args: [-strict, -summary]
    plugins:
      dry-run:
        command: helm
        args: [install, ., -g, --dry-run]
        description: invoke helm dry-run
        keys:
          - code: shift+h
            alias: ⇧+h

  ks:
    command: kustomize
    args: [build, .]
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml"])
    ui:
      compact: true
      theme: tokyonight-storm
    hooks:
      init:
        - command: kustomize
          args: [version]
```

### 🧩 CEL Functions

`kat` provides custom CEL functions for file path operations:

- `pathBase(string)`: Returns the filename (e.g., `"Chart.yaml"`)
- `pathExt(string)`: Returns the file extension (e.g., `".yaml"`)
- `pathDir(string)`: Returns the directory path
- `yamlPath(file, path)`: Reads a YAML file and extracts a value using a YAML path expression

You can combine these with CEL's built-in functions like `exists()`, `filter()`, `in`, `contains()`, `matches()`, and logical operators.

Example:

```yaml
rules:
  - match: >-
      files.exists(f,
        pathBase(f) == "Chart.yaml" &&
        yamlPath(f, "$.apiVersion") == "v2")
    profile: helm

profiles:
  helm:
    command: helm
    args: [template, ., --generate-name]
    source: >-
      files.filter(f,
        pathExt(f) in [".yaml", ".yml", ".tpl"])
```

For more details on CEL expressions and examples, see the [CEL documentation](docs/CEL.md).

### 🔥 DRY Configuration

The `kat` configuration supports YAML [anchor nodes](https://yaml.org/spec/1.2.2/#692-node-anchors), [alias nodes](https://yaml.org/spec/1.2.2/#71-alias-nodes), and [merge keys](https://yaml.org/type/merge.html). You can define common settings once and reuse them across the configuration.

```yaml
profiles:
  ks: &ks
    command: kustomize
    args: [build, .]
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml"])
    hooks:
      postRender:
        - &kubeconform
          command: kubeconform
          args: [-strict, -summary]

  ks-helm:
    <<: *ks
    args: [build, ., --enable-helm]

  helm:
    command: helm
    args: [template, ., --generate-name]
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])
    hooks:
      postRender:
        - *kubeconform
```

> ❤️ Thanks to [goccy/go-yaml](https://github.com/goccy/go-yaml).

### 📖 Examples

**Default config** - By default, `kat` includes a configuration that supports `helm`, `kustomize`, and generic YAML files. This is a great starting point for writing your own custom config:

- See [`pkg/config/config.yaml`](pkg/config/config.yaml) for the default configuration.

**Support for custom tools** - You can add support for other languages/tools like [`kcl`](https://www.kcl-lang.io/), [`jsonnet`](https://jsonnet.org/), [`flux-local`](https://github.com/allenporter/flux-local), [`cue`](https://cuelang.org/), and so on:

```yaml
rules:
  - match: >-
      files.exists(f, pathExt(f) == ".k")
    profile: kcl
profiles:
  kcl:
    command: kcl
    args: [run, .]
    source: >-
      files.filter(f, pathExt(f) == ".k")
```

**Content-based detection** - Match based on file content, not just names:

```yaml
rules:
  - # Match Helm v3 specifically
    match: >-
      files.exists(f,
        pathBase(f) == "Chart.yaml" &&
        yamlPath(f, "$.apiVersion") == "v2")
    profile: helm-v3
  - # Match Kubernetes native manifests with specific API versions
    match: >-
      files.exists(f,
        pathExt(f) in [".yaml", ".yml"] &&
        yamlPath(f, "$.apiVersion") in ["apps/v1", "v1"])
    profile: yaml
```

**Using Task** - If you use [`task`](https://taskfile.dev), you can use your tasks in the `kat` config:

```yaml
rules:
  - match: >-
      files.exists(f, pathBase(f) in ["Taskfile.yml", "Taskfile.yaml"])
    profile: task
profiles:
  task:
    command: task
    args: [render]
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml"])
    hooks:
      postRender:
        - command: task
          args: [validate]
```

> Note that you should write your `task` to:
>
> - Output the rendered manifests to stdout, and anything else to stderr.
> - Tolerate being called from any directory in the project.
>   - E.g., instead of `./folder`, use `{{joinPath .ROOT_DIR "folder"}}`.
> - Not require any additional arguments to run.
>   - You can reference `{{.USER_WORKING_DIR}}` to obtain the path that the user invoked `kat` from/with.
>   - E.g., `vars: { PATH: "{{.PATH | default .USER_WORKING_DIR}}" }`
>
> If you are concerned about safety (i.e. accidentally calling a task defined by someone else), you can consider not including a rule for `task` and only allowing it to be invoked manually via the CLI args, or you could write a more narrow match expression (e.g. `f.contains("/my-org/")`).

## 🌈 Themes

![Themes](./docs/assets/themes.gif)

Configure a theme with `--ui-theme`, `KAT_UI_THEME`, or via config:

```yaml
ui:
  theme: "dracula"
```

You can optionally set different themes for different profiles:

```yaml
profiles:
  helm:
    ui:
      theme: "dracula"
      # ...
  ks:
    ui:
      theme: "tokyonight-storm"
      # ...
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

## 🔍️ Similar Tools

- [bat](https://github.com/sharkdp/bat)
- [glow](https://github.com/charmbracelet/glow)
- [k9s](https://github.com/derailed/k9s)
- [viddy](https://github.com/sachaos/viddy)
- [soft-serve](https://github.com/charmbracelet/soft-serve)
- [wishlist](https://github.com/charmbracelet/wishlist)
