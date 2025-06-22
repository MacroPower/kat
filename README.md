<p align="center">
  <a href="#"><img src="docs/assets/logo.svg" width="200px"></a>
  <h1 align="center">kat</h1>
</p>

<p align="center">
  <code>kat</code> is like <code>cat</code> for projects that render Kubernetes manifests. It provides a pretty terminal UI to quickly <b>find</b>, <b>debug</b>, and <b>reload</b> manifests, without ever leaving your shell.
</p>

<p align="center">
  <code>kat</code> is designed to reduce inner loop time for <b>developers</b> and <b>platform engineers</b> working on things like <code>helm</code> charts and <code>kustomize</code> projects. By editing the config, you can make <code>kat</code> work with anything that generates Kubernetes manifests, with powerful content-based project detection!
</p>

<p align="center">
  <br>
  <img src="./docs/assets/demo.gif">
</p>

<p align="center">
  ❤️ Made with <a href="https://github.com/charmbracelet/bubbletea">bubble tea</a>, <a href="https://github.com/charmbracelet/glow">glow</a>, and <a href="https://github.com/alecthomas/chroma/">chroma</a>.
</p>

## ✨ Features

- 🚀 List and filter hundreds of manifests without leaving your shell.
- 🔄 Reload from any context to quickly diff individual manifests.
- 👀 Use `--watch` to trigger reloads on changes to source files.
- 🐛 Immediately view any errors from rendering, and re-reload!
- 🎨 Customize keybinds, styles, and more to match your preferences.
- 🪄 Add your own rules to detect different project types.
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

Alternatively, you can use `kat --show-config` to print the active configuration and redirect the output to a file.

You can also find an example configuration file in [pkg/config/config.yaml](pkg/config/config.yaml).

## 🛠️ Rules and Profiles

You can customize how `kat` detects and renders different types of projects using **rules** and **profiles** in the configuration file. This system uses [CEL (Common Expression Language)](https://cel.dev/) expressions to provide flexible file matching and processing.

### Rules

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

### Profiles

**Profiles define how to render projects.** They can be automatically selected by rules, or manually specified when `kat` is invoked. Each profile contains:

- `command` (required): The command to execute
- `args`: Arguments to pass to the command
- `source`: Define which files to watch for changes (when watch is enabled)
- `theme`: UI theme to use for this profile
- `hooks`: Initialization and rendering hooks
  - `init` hooks are executed once when `kat` is initialized
  - `preRender` hooks are executed before the profile's command is run
  - `postRender` hooks are executed after the profile's command has run, and are provided the rendered output via stdin

Profile `source` expressions use list-returning CEL expressions with the same variables as rules.

```yaml
profiles:
  helm:
    command: helm
    args: [template, ., --generate-name]
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])
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

  ks:
    command: kustomize
    args: [build, .]
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml"])
    theme: tokyonight-storm
    hooks:
      init:
        - command: kustomize
          args: [version]
```

### CEL Functions

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

### 📖 Examples

**Default config** - By default, `kat` includes a configuration that supports `helm`, `kustomize`, and generic YAML files. This is a great starting point for writing your own custom config:

```yaml
apiVersion: kat.jacobcolvin.com/v1beta1
kind: Configuration
rules:
  - # Select the Kustomize profile if Kustomization files exist
    match: >-
      files.exists(f,
        pathBase(f) in ["kustomization.yaml", "kustomization.yml"])
    profile: ks
  - # Select the Helm profile if Helm chart files exist
    match: >-
      files.exists(f,
        pathBase(f) in ["Chart.yaml", "Chart.yml"])
    profile: helm
  - # Fallback: select the YAML profile if any YAML files exist
    match: >-
      files.exists(f,
        pathExt(f) in [".yaml", ".yml"])
    profile: yaml
profiles:
  helm:
    command: helm
    args: [template, ., --generate-name]
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml", ".tpl"])
    hooks:
      init:
        - command: helm
          args: [version, --short]
      preRender:
        - command: helm
          args: [dependency, build]
  ks:
    command: kustomize
    args: [build, .]
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml"])
    hooks:
      init:
        - command: kustomize
          args: [version]
  yaml:
    command: sh
    args:
      - -c
      - yq eval-all '.' *.yaml
    source: >-
      files.filter(f, pathExt(f) in [".yaml", ".yml"])
    hooks:
      init:
        - command: yq
          args: [-V]
```

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
    theme: "dracula"
    # ...
  ks:
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
