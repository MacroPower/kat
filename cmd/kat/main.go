package main

import (
	"io"
	"log/slog"
	"path/filepath"

	"github.com/alecthomas/kong"

	"github.com/MacroPower/kat/pkg/log"
	"github.com/MacroPower/kat/pkg/ui"
	"github.com/MacroPower/kat/pkg/version"
)

const appName = "kat"

var cli struct {
	Log struct {
		Level  string `default:"info"   help:"Log level."`
		Format string `default:"logfmt" help:"Log format. One of: [logfmt, json]"`
	} `embed:"" prefix:"log."`
}

func main() {
	cliCtx := kong.Parse(&cli, kong.Name(appName))

	logHandler, err := log.CreateHandlerWithStrings(cliCtx.Stderr, cli.Log.Level, cli.Log.Format)
	if err != nil {
		cliCtx.FatalIfErrorf(err)
	}
	slog.SetDefault(slog.New(logHandler))

	slog.Info("starting",
		slog.String("app", appName),
		slog.String("v", version.Version),
		slog.String("revision", version.Revision),
	)

	path, err := filepath.Abs(".")
	if err != nil {
		cliCtx.FatalIfErrorf(err)
	}

	p := ui.NewProgram(ui.Config{Path: path, ShowAllFiles: true, GlamourEnabled: true, GlamourStyle: "dark"}, "")
	if _, err := p.Run(); err != nil {
		cliCtx.Fatalf("tea: %v", err)
	}
}

func Hello(r io.Writer) {
	_, err := r.Write([]byte("Hello World!"))
	if err != nil {
		panic(err)
	}
}
