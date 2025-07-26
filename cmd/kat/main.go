package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"

	"github.com/macropower/kat/internal/cli"
	"github.com/macropower/kat/pkg/version"
)

func main() {
	ctx := context.Background()
	err := fang.Execute(ctx, cli.NewRootCmd(),
		fang.WithVersion(version.GetVersion()),
		fang.WithCommit(version.Revision),
		fang.WithErrorHandler(cli.ErrorHandler),
		fang.WithColorSchemeFunc(cli.ColorSchemeFunc),
	)
	if err != nil {
		os.Exit(1)
	}
}
