package common

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/keys"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/ui/statusbar"
	"github.com/macropower/kat/pkg/ui/theme"
)

type Commander interface {
	Run() command.Output
	RunContext(ctx context.Context) command.Output
	RunOnEvent()
	String() string
	Subscribe(ch chan<- command.Event)
	GetProfiles() map[string]*profile.Profile
	GetCurrentProfile() (string, *profile.Profile)
	FindProfiles(path string) ([]command.ProfileMatch, error)
	Configure(opts ...command.RunnerOpt) error
	RunPlugin(name string) command.Output
	FS() (*command.FilteredFS, error)
}

type CommonModel struct {
	Cmd                Commander
	Theme              *theme.Theme
	StatusMessageTimer *time.Timer
	KeyBinds           *KeyBinds
	StatusMessage      StatusMessage
	Width              int
	Height             int
	Loaded             bool
	ShowStatusMessage  bool // Whether to show the status message in the status bar.
}

const StatusMessageTimeout = time.Second * 3 // How long to show status messages.

type (
	StatusMessage struct {
		Message string
		Style   statusbar.Style
	}
	StatusMessageTimeoutMsg struct{}
)

func (m *CommonModel) GetStatusBar() *statusbar.StatusBarRenderer {
	if m.ShowStatusMessage && m.StatusMessage.Message != "" {
		return statusbar.NewStatusBarRenderer(m.Theme, m.Width,
			statusbar.WithMessage(m.StatusMessage.Message, m.StatusMessage.Style))
	}

	return statusbar.NewStatusBarRenderer(m.Theme, m.Width)
}

// Show a status (success) message to the user.
func (m *CommonModel) SendStatusMessage(msg string, style statusbar.Style) tea.Cmd {
	m.ShowStatusMessage = true
	m.StatusMessage = StatusMessage{
		Message: msg,
		Style:   style,
	}
	if m.StatusMessageTimer != nil {
		m.StatusMessageTimer.Stop()
	}

	m.StatusMessageTimer = time.NewTimer(StatusMessageTimeout)

	return WaitForStatusMessageTimeout(m.StatusMessageTimer)
}

type ErrMsg struct{ Err error } //nolint:errname // Tea message.

func (e ErrMsg) Error() string { return e.Err.Error() }

func WaitForStatusMessageTimeout(t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C

		return StatusMessageTimeoutMsg{}
	}
}

type KeyBinds struct {
	Quit    *keys.KeyBind `json:"quit,omitempty"`
	Suspend *keys.KeyBind `json:"suspend,omitempty"`
	Reload  *keys.KeyBind `json:"reload,omitempty"`
	Help    *keys.KeyBind `json:"help,omitempty"`
	Error   *keys.KeyBind `json:"error,omitempty"`
	Escape  *keys.KeyBind `json:"escape,omitempty"`
	Menu    *keys.KeyBind `json:"menu,omitempty"`

	// Navigation.
	Up    *keys.KeyBind `json:"up,omitempty"`
	Down  *keys.KeyBind `json:"down,omitempty"`
	Left  *keys.KeyBind `json:"left,omitempty"`
	Right *keys.KeyBind `json:"right,omitempty"`
	Prev  *keys.KeyBind `json:"prev,omitempty"`
	Next  *keys.KeyBind `json:"next,omitempty"`
}

func (kb *KeyBinds) EnsureDefaults() {
	keys.SetDefaultBind(&kb.Quit, keys.NewBind("quit", keys.New("q")))
	// Always ensure that ctrl+c is bound to quit.
	kb.Quit.AddKey(keys.New("ctrl+c", keys.WithAlias("⌃c"), keys.Hidden()))

	keys.SetDefaultBind(&kb.Suspend,
		keys.NewBind("suspend",
			keys.New("ctrl+z", keys.WithAlias("⌃z"), keys.Hidden()),
		))
	keys.SetDefaultBind(&kb.Reload,
		keys.NewBind("reload",
			keys.New("r"),
		))
	keys.SetDefaultBind(&kb.Escape,
		keys.NewBind("go back",
			keys.New("esc"),
		))
	keys.SetDefaultBind(&kb.Help,
		keys.NewBind("toggle help",
			keys.New("?"),
		))
	keys.SetDefaultBind(&kb.Error,
		keys.NewBind("toggle error",
			keys.New("!"),
		))
	keys.SetDefaultBind(&kb.Menu,
		keys.NewBind("open menu",
			keys.New(":"),
		))

	keys.SetDefaultBind(&kb.Up,
		keys.NewBind("move up",
			keys.New("up", keys.WithAlias("↑")),
			keys.New("k"),
		))
	keys.SetDefaultBind(&kb.Down,
		keys.NewBind("move down",
			keys.New("down", keys.WithAlias("↓")),
			keys.New("j"),
		))
	keys.SetDefaultBind(&kb.Left,
		keys.NewBind("move left",
			keys.New("left", keys.WithAlias("←")),
			keys.New("h"),
		))
	keys.SetDefaultBind(&kb.Right,
		keys.NewBind("move right",
			keys.New("right", keys.WithAlias("→")),
			keys.New("l"),
		))
	keys.SetDefaultBind(&kb.Prev,
		keys.NewBind("previous page",
			keys.New("shift+tab", keys.WithAlias("⇧+tab")),
			keys.New("H"),
		))
	keys.SetDefaultBind(&kb.Next,
		keys.NewBind("next page",
			keys.New("tab"),
			keys.New("L"),
		))
}

func (kb *KeyBinds) GetKeyBinds() []keys.KeyBind {
	return []keys.KeyBind{
		*kb.Quit,
		*kb.Suspend,
		*kb.Reload,
		*kb.Escape,
		*kb.Help,
		*kb.Error,
		*kb.Menu,
		*kb.Up,
		*kb.Down,
		*kb.Left,
		*kb.Right,
		*kb.Prev,
		*kb.Next,
	}
}
