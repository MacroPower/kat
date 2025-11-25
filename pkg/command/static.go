package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/macropower/kat/pkg/kube"
	"github.com/macropower/kat/pkg/log"
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

func (rg *Static) ConfigureContext(_ context.Context, _ ...RunnerOpt) error {
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

func (rg *Static) GetPath() string {
	return "."
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
	return rg.RunContext(context.Background())
}

func (rg *Static) RunContext(ctx context.Context) Output {
	rg.broadcast(NewEventStart(ctx, TypeRun))

	out := NewOutput(TypeRun)
	if rg.Resources == nil {
		out.Error = errors.New("no resources available")
	}

	out.Resources = rg.Resources

	rg.broadcast(NewEventEnd(ctx, out))

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
	ctx := evt.GetContext()

	log.WithContext(ctx).DebugContext(ctx, "broadcasting event",
		slog.String("event", fmt.Sprintf("%T", evt)),
	)

	for _, ch := range rg.listeners {
		ch <- evt
	}
}

// SendEvent allows external components to send events to all listeners.
func (rg *Static) SendEvent(evt Event) {
	rg.broadcast(evt)
}

func (rg *Static) RunPlugin(_ string) Output {
	return rg.RunPluginContext(context.Background(), "")
}

func (rg *Static) RunPluginContext(ctx context.Context, _ string) Output {
	rg.broadcast(NewEventStart(ctx, TypePlugin))

	out := NewOutput(TypePlugin, WithError(errors.New("plugins not supported in static resource mode")))

	rg.broadcast(NewEventEnd(ctx, out))

	return out
}

func (rg *Static) GetRules() []*rule.Rule {
	return nil
}

func (rg *Static) FS() (*FilteredFS, error) {
	return NewFilteredFSFromPath(os.TempDir())
}
