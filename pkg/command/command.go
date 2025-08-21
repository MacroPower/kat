package command

import (
	"time"

	"github.com/macropower/kat/pkg/kube"
)

type Type int

const (
	// TypeRun indicates a command execution.
	TypeRun Type = iota
	// TypePlugin indicates a plugin execution.
	TypePlugin
)

type Output struct {
	Timestamp time.Time
	Error     error
	Stdout    string
	Stderr    string
	Resources []*kube.Resource
	Type      Type
}

// NewOutput creates a new [Output] timestamped with the current time.
func NewOutput(t Type, opts ...OutputOpt) Output {
	o := &Output{
		Type:      t,
		Timestamp: time.Now(),
	}
	for _, opt := range opts {
		opt(o)
	}

	return *o
}

type OutputOpt func(*Output)

// WithError sets the error for the output.
func WithError(err error) OutputOpt {
	return func(o *Output) {
		o.Error = err
	}
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

	// EventOpenResource indicates that a specific resource was opened.
	EventOpenResource kube.Resource

	// EventListResources indicates that a list of resources was requested.
	EventListResources struct{}
)
