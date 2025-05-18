package main

import (
	"io"
	"log/slog"
	"path/filepath"

	"github.com/alecthomas/kong"

	"github.com/MacroPower/kat/pkg/kube"
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

	path, err := filepath.Abs("./example/kustomize")
	if err != nil {
		cliCtx.FatalIfErrorf(err)
	}

	slog.Info("starting",
		slog.String("app", appName),
		slog.String("v", version.Version),
		slog.String("revision", version.Revision),
		slog.String("path", path),
	)

	cmd := kube.NewCommandRunner(path)

	p := ui.NewProgram(ui.Config{GlamourEnabled: true, GlamourStyle: "dark"}, cmd)
	if _, err := p.Run(); err != nil {
		cliCtx.Fatalf("tea: %v", err)
	}
}
