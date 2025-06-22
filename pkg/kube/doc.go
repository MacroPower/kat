// Package kube provides functionality for orchestrating command execution
// against Kubernetes resources with file watching and YAML processing.
//
// This package serves as the main orchestrator, combining functionality from:
//   - pkg/expr: CEL expression evaluation
//   - pkg/rules: File matching rules
//   - pkg/profiles: Command execution profiles
//
// It provides the CommandRunner for managing file-to-command mappings,
// filesystem notifications, concurrent command execution, and YAML resource
// processing specific to Kubernetes workflows.
package kube
