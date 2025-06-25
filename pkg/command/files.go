package command

import (
	"errors"
	"fmt"

	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/profile"
)

type ResourceGetter struct {
	Resources []*kube.Resource
	listeners []chan<- Event
}

func NewResourceGetter(input string) (*ResourceGetter, error) {
	if input == "" {
		return nil, errors.New("input cannot be empty")
	}

	resources, err := kube.SplitYAML([]byte(input))
	if err != nil {
		return nil, fmt.Errorf("split yaml: %w", err)
	}

	return &ResourceGetter{Resources: resources}, nil
}

func (rg *ResourceGetter) String() string {
	return "static"
}

func (rg *ResourceGetter) GetCurrentProfile() *profile.Profile {
	// Static resources do not have a profile.
	return &profile.Profile{}
}

func (rg *ResourceGetter) Run() Output {
	rg.broadcast(EventStart(TypeRun))

	out := Output{
		Type:      TypeRun,
		Resources: rg.Resources,
	}
	if rg.Resources == nil {
		out.Error = errors.New("no resources available")
	}

	rg.broadcast(EventEnd(out))

	return out
}

func (rg *ResourceGetter) RunOnEvent() {
	// No events to watch for in static resources.
}

func (rg *ResourceGetter) Close() {
	// No resources to close.
}

func (rg *ResourceGetter) Subscribe(ch chan<- Event) {
	rg.listeners = append(rg.listeners, ch)
}

func (rg *ResourceGetter) broadcast(evt Event) {
	// Send the event to all listeners.
	for _, ch := range rg.listeners {
		ch <- evt
	}
}

func (rg *ResourceGetter) RunPlugin(_ string) Output {
	rg.broadcast(EventStart(TypePlugin))

	out := Output{
		Type:  TypePlugin,
		Error: errors.New("plugins not supported in static resource mode"),
	}

	rg.broadcast(EventEnd(out))

	return out
}
