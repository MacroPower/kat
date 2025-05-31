package common

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/ui/config"
	"github.com/MacroPower/kat/pkg/ui/statusbar"
	"github.com/MacroPower/kat/pkg/ui/styles"
)

type Commander interface {
	Run() (kube.CommandOutput, error)
	String() string
}

type CommonModel struct {
	Cmd                Commander
	StatusMessageTimer *time.Timer
	StatusMessage      StatusMessage
	Config             config.Config
	Width              int
	Height             int
}

// ApplicationContext indicates the area of the application something applies
// to. Occasionally used as an argument to commands and messages.
type ApplicationContext int

const (
	StashContext ApplicationContext = iota
	PagerContext

	StatusMessageTimeout = time.Second * 3 // How long to show status messages.
)

type RunOutput struct {
	Err error
	Out kube.CommandOutput
}

type (
	StatusMessage struct {
		Message string
		IsError bool
	}
	StatusMessageTimeoutMsg ApplicationContext

	CommandRunStarted struct {
		Ch chan RunOutput
	}
	CommandRunFinished RunOutput
)

func (m *CommonModel) GetStatusBar(showMessage bool) *statusbar.StatusBarRenderer {
	if showMessage && m.StatusMessage.IsError {
		return statusbar.NewStatusBarRenderer(m.Width, statusbar.WithError(m.StatusMessage.Message))
	}

	if showMessage && m.StatusMessage.Message != "" {
		return statusbar.NewStatusBarRenderer(m.Width, statusbar.WithMessage(m.StatusMessage.Message))
	}

	return statusbar.NewStatusBarRenderer(m.Width)
}

// Lightweight version of reflow's indent function.
func Indent(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}
	l := strings.Split(s, "\n")
	b := strings.Builder{}
	i := strings.Repeat(" ", n)
	for _, v := range l {
		fmt.Fprintf(&b, "%s%s\n", i, v)
	}

	return b.String()
}

type ErrMsg struct{ Err error } //nolint:errname // Tea message.

func (e ErrMsg) Error() string { return e.Err.Error() }

func ErrorView(err string, fatal bool) string {
	exitMsg := "press any key to "
	if fatal {
		exitMsg += "exit"
	} else {
		exitMsg += "return"
	}
	s := fmt.Sprintf("%s\n\n%s\n\n%s",
		styles.ErrorTitleStyle.Render("ERROR"),
		err,
		styles.SubtleStyle.Render(exitMsg),
	)

	return "\n" + Indent(s, 3)
}

func WaitForStatusMessageTimeout(appCtx ApplicationContext, t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C

		return StatusMessageTimeoutMsg(appCtx)
	}
}
