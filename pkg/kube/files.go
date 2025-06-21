package kube

import (
	"errors"
	"fmt"
)

type ResourceGetter struct {
	Resources []*Resource
	listeners []chan<- CommandEvent
}

func NewResourceGetter(input string) (*ResourceGetter, error) {
	if input == "" {
		return nil, errors.New("input cannot be empty")
	}

	resources, err := SplitYAML([]byte(input))
	if err != nil {
		return nil, fmt.Errorf("split yaml: %w", err)
	}

	return &ResourceGetter{Resources: resources}, nil
}

func (rg *ResourceGetter) String() string {
	return "static"
}

func (rg *ResourceGetter) GetCurrentTheme() string {
	// Static resources do not have a theme.
	return ""
}

func (rg *ResourceGetter) Run() CommandOutput {
	rg.broadcast(CommandEventStart{})

	out := CommandOutput{Resources: rg.Resources}
	if rg.Resources == nil {
		out.Error = errors.New("no resources available")
	}

	rg.broadcast(CommandEventEnd(out))

	return out
}

func (rg *ResourceGetter) RunOnEvent() {
	// No events to watch for in static resources.
}

func (rg *ResourceGetter) Close() {
	// No resources to close.
}

func (rg *ResourceGetter) Subscribe(ch chan<- CommandEvent) {
	rg.listeners = append(rg.listeners, ch)
}

func (rg *ResourceGetter) broadcast(evt CommandEvent) {
	// Send the event to all listeners.
	for _, ch := range rg.listeners {
		ch <- evt
	}
}
