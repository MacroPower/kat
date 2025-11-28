# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`kat` is a rule-based rendering engine and terminal UI for Kubernetes manifests. It automatically invokes manifest generators (Helm, Kustomize, etc.) and provides a navigable view of rendered resources with live reloading, validation, and an optional MCP server.

## Build & Development Commands

```bash
# Format code
task format

# Lint all (Go, YAML, Actions, Renovate, GoReleaser)
task lint

# Run tests
task test

# Run a single test
go test ./pkg/config -run TestConfigLoader
```

## Architecture

### Entry Point & CLI

- `cmd/kat/main.go` - Entry point using charmbracelet/fang
- `internal/cli/` - CLI commands, flags, and environment binding

### Core Packages (`pkg/`)

**Rendering Pipeline:**

- `pkg/command/` - Command execution orchestration, file watching, event broadcasting
- `pkg/profile/` - Profile definitions with CEL-based source/reload expressions, hooks, plugins
- `pkg/rule/` - CEL-based rule matching for automatic profile selection
- `pkg/expr/` - CEL expression compilation and evaluation with custom functions (pathBase, pathExt, yamlPath, etc.)

**Kubernetes & YAML:**

- `pkg/kube/` - Kubernetes resource parsing and object representation
- `pkg/yaml/` - YAML encoding/decoding, validation, error formatting

**UI (Bubble Tea):**

- `pkg/ui/` - Main TUI program and state management
- `pkg/ui/list/` - Resource list with fuzzy filtering
- `pkg/ui/pager/` - YAML document viewer with syntax highlighting
- `pkg/ui/menu/` - Configuration menu for profile/path changes
- `pkg/ui/overlay/` - Error/loading overlays
- `pkg/ui/theme/` - Chroma-based theming

**Other:**

- `pkg/config/` - Configuration loading, validation, and default config embedding
- `pkg/mcp/` - Model Context Protocol server for AI integrations
- `pkg/execs/` - Command execution utilities
- `pkg/keys/` - Keybinding definitions

### Data Flow

1. CLI parses args and loads config from `$XDG_CONFIG_HOME/kat/config.yaml`
2. `command.Runner` matches path against rules to select a profile
3. Profile executes command with hooks (init, preRender, postRender)
4. Rendered YAML is parsed into `kube.Resource` objects
5. Events broadcast to UI and MCP server subscribers
6. UI displays resources in list/pager views

## Code Style

### Go Conventions

- Document all exported items with doc comments
- Use `[Name]` syntax for Go doc links
- Package documentation in `doc.go` files
- Wrap errors with `fmt.Errorf("context: %w", err)` - no "failed" or "error" in messages
- Use global error variables for common errors

### Testing

- Use `github.com/stretchr/testify/assert` and `require`
- Table-driven tests with `map[string]struct{}` format
- Field names: `input`, `want`, `got`, `err`
- Always use `t.Parallel()` in all tests
- Create test packages (`package foo_test`) testing public API
- Use `require.ErrorIs` for error type checking

## Key Dependencies

- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/google/cel-go` - CEL expression engine
- `github.com/alecthomas/chroma/v2` - Syntax highlighting
- `github.com/goccy/go-yaml` - YAML with anchors/aliases support
- `github.com/fsnotify/fsnotify` - File watching
- `github.com/modelcontextprotocol/go-sdk` - MCP server
