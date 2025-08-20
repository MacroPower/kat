package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

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

func (rg *Static) Configure(_ ...RunnerOpt) error {
	return nil
}

func (rg *Static) String() string {
	return "static"
}

func (rg *Static) GetCurrentProfile() (string, *profile.Profile) {
	// Static resources do not have a profile.
	return "static", &profile.Profile{}
}

func (rg *Static) GetProfiles() map[string]*profile.Profile {
	return map[string]*profile.Profile{}
}

func (rg *Static) FindProfile(_ string) (string, *profile.Profile, error) {
	// Static resources do not have a profile.
	return "", nil, errors.ErrUnsupported
}

func (rg *Static) FindProfiles(_ string) ([]ProfileMatch, error) {
	// Static resources do not have a profile.
	return nil, errors.ErrUnsupported
}

func (rg *Static) Run() Output {
	rg.broadcast(EventStart(TypeRun))

	out := NewOutput(TypeRun)
	if rg.Resources == nil {
		out.Error = errors.New("no resources available")
	}

	out.Resources = rg.Resources

	rg.broadcast(EventEnd(out))

	return out
}

func (rg *Static) RunContext(_ context.Context) Output {
	return rg.Run()
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
	slog.Debug("broadcasting event",
		slog.String("event", fmt.Sprintf("%T", evt)),
	)

	for _, ch := range rg.listeners {
		ch <- evt
	}
}

func (rg *Static) RunPlugin(_ string) Output {
	rg.broadcast(EventStart(TypePlugin))

	out := NewOutput(TypePlugin, WithError(errors.New("plugins not supported in static resource mode")))

	rg.broadcast(EventEnd(out))

	return out
}

func (rg *Static) GetRules() []*rule.Rule {
	return nil
}

func (rg *Static) FS() (*FilteredFS, error) {
	return NewFilteredFS(os.TempDir())
}
