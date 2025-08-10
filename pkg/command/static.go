package command

import (
	"errors"
	"fmt"

	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/rule"
)

type Static struct {
	Resources []*kube.Resource
	listeners []chan<- Event
}

func NewStatic(input string) (*Static, error) {
	if input == "" {
		return nil, errors.New("input cannot be empty")
	}

	resources, err := kube.SplitYAML([]byte(input))
	if err != nil {
		return nil, fmt.Errorf("split yaml: %w", err)
	}

	return &Static{Resources: resources}, nil
}

func (rg *Static) String() string {
	return "static"
}

func (rg *Static) GetCurrentProfile() *profile.Profile {
	// Static resources do not have a profile.
	return &profile.Profile{}
}

func (rg *Static) Run() Output {
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

func (rg *Static) RunOnEvent() {
	// No events to watch for in static resources.
}

func (rg *Static) Close() {
	// No resources to close.
}

func (rg *Static) Subscribe(ch chan<- Event) {
	rg.listeners = append(rg.listeners, ch)
}

func (rg *Static) broadcast(evt Event) {
	// Send the event to all listeners.
	for _, ch := range rg.listeners {
		ch <- evt
	}
}

func (rg *Static) RunPlugin(_ string) Output {
	rg.broadcast(EventStart(TypePlugin))

	out := Output{
		Type:  TypePlugin,
		Error: errors.New("plugins not supported in static resource mode"),
	}

	rg.broadcast(EventEnd(out))

	return out
}

func (rg *Static) GetRules() []*rule.Rule {
	return nil
}

func (rg *Static) FS() (*FilteredFS, error) {
	return nil, errors.ErrUnsupported
}
