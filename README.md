# kat

`cat` for Kubernetes resources. Uses [bubbletea](https://github.com/charmbracelet/bubbletea) and code from [glow](https://github.com/charmbracelet/glow).

```sh
# Chart or Kustomization in the current directory
kat .

# Resources from stdin
kustomize ./example | kat

# Output from a command (with watch/reload)
kat -- helm template .
```

## TODO

- `/` to find inside the YAML viewer.
- vim/k9s-like command mode.
- watch/reload `kat -- helm template . -g --debug`

## Similar Tools

- [bat](https://github.com/sharkdp/bat)
- [glow](https://github.com/charmbracelet/glow)
- [k9s](https://github.com/derailed/k9s)
- [viddy](https://github.com/sachaos/viddy)
