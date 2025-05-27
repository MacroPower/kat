package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/MacroPower/kat/pkg/kube"
	"github.com/MacroPower/kat/pkg/ui/config"
	"github.com/MacroPower/kat/pkg/ui/styles"
	"github.com/MacroPower/kat/pkg/version"
)

type Commander interface {
	Run() (kube.CommandOutput, error)
	String() string
}

type CommonModel struct {
	Cmd    Commander
	Config config.Config
	Width  int
	Height int
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
	StatusMessageTimeoutMsg ApplicationContext

	CommandRunStarted struct {
		Ch chan RunOutput
	}
	CommandRunFinished RunOutput
)

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

func ErrorView(err error, fatal bool) string {
	exitMsg := "press any key to "
	if fatal {
		exitMsg += "exit"
	} else {
		exitMsg += "return"
	}
	s := fmt.Sprintf("%s\n\n%v\n\n%s",
		styles.ErrorTitleStyle.Render("ERROR"),
		err,
		styles.SubtleStyle.Render(exitMsg),
	)

	return "\n" + Indent(s, 3)
}

func KatLogoView() string {
	return styles.LogoStyle.Render(fmt.Sprintf(" kat %s ", version.GetVersion()))
}
