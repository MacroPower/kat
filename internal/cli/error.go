package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/lipgloss"
)

func ErrorHandler(w io.Writer, styles fang.Styles, err error) {
	mustN(fmt.Fprintln(w, styles.ErrorHeader.String()))
	mustN(fmt.Fprintln(w, lipgloss.NewStyle().MarginLeft(2).Render(err.Error())))
	mustN(fmt.Fprintln(w))
	if isUsageError(err) {
		mustN(fmt.Fprintln(w, lipgloss.JoinHorizontal(
			lipgloss.Left,
			styles.ErrorText.UnsetWidth().Render("Try"),
			styles.Program.Flag.Render("--help"),
			styles.ErrorText.UnsetWidth().UnsetMargins().UnsetTransform().PaddingLeft(1).Render("for usage."),
		)))
		mustN(fmt.Fprintln(w))
	}
}

// XXX: this is a hack to detect usage errors.
// See: https://github.com/spf13/cobra/pull/2266
func isUsageError(err error) bool {
	s := err.Error()
	for _, prefix := range []string{
		"flag needs an argument:",
		"unknown flag:",
		"unknown shorthand flag:",
		"unknown command",
		"invalid argument",
	} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}

	return false
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func mustN(_ int, err error) {
	must(err)
}
