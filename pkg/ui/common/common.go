package common

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/ui/config"
	"github.com/MacroPower/kat/pkg/ui/statusbar"
)

type Commander interface {
	Run() kube.CommandOutput
	RunOnUpdate(ch chan<- kube.CommandOutput)
	String() string
}

type CommonModel struct {
	Cmd                Commander
	StatusMessageTimer *time.Timer
	StatusMessage      StatusMessage
	Config             config.Config
	Width              int
	Height             int
	Loaded             bool
	ShowStatusMessage  bool // Whether to show the status message in the status bar.
}

// ApplicationContext indicates the area of the application something applies
// to. Occasionally used as an argument to commands and messages.
type ApplicationContext int

const (
	StashContext ApplicationContext = iota
	PagerContext

	StatusMessageTimeout = time.Second * 3 // How long to show status messages.
)

type (
	StatusMessage struct {
		Message string
		Style   statusbar.Style
	}
	StatusMessageTimeoutMsg ApplicationContext

	CommandRunStarted  struct{}
	CommandRunFinished kube.CommandOutput
)

func (m *CommonModel) GetStatusBar() *statusbar.StatusBarRenderer {
	if m.ShowStatusMessage && m.StatusMessage.Message != "" {
		return statusbar.NewStatusBarRenderer(m.Width,
			statusbar.WithMessage(m.StatusMessage.Message, m.StatusMessage.Style))
	}

	return statusbar.NewStatusBarRenderer(m.Width)
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

	return WaitForStatusMessageTimeout(StashContext, m.StatusMessageTimer)
}

type ErrMsg struct{ Err error } //nolint:errname // Tea message.

func (e ErrMsg) Error() string { return e.Err.Error() }

func WaitForStatusMessageTimeout(appCtx ApplicationContext, t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C

		return StatusMessageTimeoutMsg(appCtx)
	}
}
