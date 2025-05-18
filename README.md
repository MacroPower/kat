# kat

`cat` for Kubernetes manifests. Uses [bubbletea](https://github.com/charmbracelet/bubbletea) and code from [glow](https://github.com/charmbracelet/glow).

```sh
# kat the current directory.
kat .

# kat a file or directory path.
kat ./example/kustomize

# kat with command passthrough.
kat ./example/helm -- helm template foobar .
```

Automatically detects and renders `helm` and `kustomize` projects by default, can be extended via `$XDG_CONFIG_HOME/kat/config.yaml`.

## TODO

- Support manifests from stdin.
- Watch/reload (i.e. re-run commands automatically).
- Vim keybindings inside the YAML viewer.

## Similar Tools

- [bat](https://github.com/sharkdp/bat)
- [glow](https://github.com/charmbracelet/glow)
- [k9s](https://github.com/derailed/k9s)
- [viddy](https://github.com/sachaos/viddy)
