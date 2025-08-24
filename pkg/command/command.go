package command

import (
	"context"
	"time"

	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/profile"
)

type Commander interface {
	Run() Output
	RunContext(ctx context.Context) Output
	RunOnEvent()
	String() string
	Subscribe(ch chan<- Event)
	GetProfiles() map[string]*profile.Profile
	GetCurrentProfile() (string, *profile.Profile)
	FindProfiles(path string) ([]ProfileMatch, error)
	Configure(opts ...RunnerOpt) error
	ConfigureContext(ctx context.Context, opts ...RunnerOpt) error
	RunPlugin(name string) Output
	RunPluginContext(ctx context.Context, name string) Output
	FS() (*FilteredFS, error)
	SendEvent(evt Event)
}

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
type Event interface {
	GetContext() context.Context
}

// EventStart indicates that a command execution has started.
type EventStart struct {
	context context.Context
	Type    Type
}

// NewEventStart creates a new EventStart with the given context and type.
func NewEventStart(ctx context.Context, t Type) EventStart {
	return EventStart{context: ctx, Type: t}
}

// GetContext returns the context associated with the EventStart.
func (e EventStart) GetContext() context.Context {
	return e.context
}

// EventEnd indicates that a command execution has ended.
// This event carries the output of the command execution, which could be
// an error if the command failed.
type EventEnd struct {
	context context.Context
	Output  Output
}

// NewEventEnd creates a new EventEnd with the given context and output.
func NewEventEnd(ctx context.Context, output Output) EventEnd {
	return EventEnd{context: ctx, Output: output}
}

// GetContext returns the context associated with the EventEnd.
func (e EventEnd) GetContext() context.Context {
	return e.context
}

// EventCancel indicates that a command execution has been canceled.
type EventCancel struct {
	context context.Context
}

// NewEventCancel creates a new EventCancel with the given context.
func NewEventCancel(ctx context.Context) EventCancel {
	return EventCancel{context: ctx}
}

// GetContext returns the context associated with the EventCancel.
func (e EventCancel) GetContext() context.Context {
	return e.context
}

// EventConfigure indicates that a command has been configured (or re-configured).
type EventConfigure struct {
	context context.Context
}

// NewEventConfigure creates a new EventConfigure with the given context.
func NewEventConfigure(ctx context.Context) EventConfigure {
	return EventConfigure{context: ctx}
}

// GetContext returns the context associated with the EventConfigure.
func (e EventConfigure) GetContext() context.Context {
	return e.context
}

// EventOpenResource indicates that a specific resource was opened.
type EventOpenResource struct {
	context  context.Context
	Resource kube.Resource
}

// NewEventOpenResource creates a new EventOpenResource with the given context and resource.
func NewEventOpenResource(ctx context.Context, resource kube.Resource) EventOpenResource {
	return EventOpenResource{context: ctx, Resource: resource}
}

// GetContext returns the context associated with the EventOpenResource.
func (e EventOpenResource) GetContext() context.Context {
	return e.context
}

// EventListResources indicates that a list of resources was requested.
type EventListResources struct {
	context context.Context
}

// NewEventListResources creates a new EventListResources with the given context.
func NewEventListResources(ctx context.Context) EventListResources {
	return EventListResources{context: ctx}
}

// GetContext returns the context associated with the EventListResources.
func (e EventListResources) GetContext() context.Context {
	return e.context
}
