package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"go.jacobcolvin.com/niceyaml/fangs"
	"go.jacobcolvin.com/x/version"

	"github.com/macropower/kat/internal/cli"
)

func main() {
	ctx := context.Background()
	err := fang.Execute(ctx, cli.NewRootCmd(),
		fang.WithVersion(version.GetVersion()),
		fang.WithCommit(version.Revision),
		fang.WithErrorHandler(fangs.ErrorHandler),
		fang.WithColorSchemeFunc(fangs.ColorSchemeFunc(cli.LoadStyles())),
	)
	if err != nil {
		os.Exit(1)
	}
}
