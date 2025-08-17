package command

import "github.com/macropower/kat/pkg/kube"

type Type int

const (
	// TypeRun indicates a command execution.
	TypeRun Type = iota
	// TypePlugin indicates a plugin execution.
	TypePlugin
)

type Output struct {
	Error     error
	Stdout    string
	Stderr    string
	Resources []*kube.Resource
	Type      Type
}

// Event represents an event related to command execution.
type Event any

type (
	// EventStart indicates that a command execution has started.
	EventStart Type

	// EventEnd indicates that a command execution has ended.
	// This event carries the output of the command execution, which could be
	// an error if the command failed.
	EventEnd Output

	// EventCancel indicates that a command execution has been canceled.
	EventCancel struct{}

	// EventConfigure indicates that a command has been configured (or re-configured).
	EventConfigure struct{}
)
