package main

import (
	"io"
	"log/slog"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/MacroPower/kat/pkg/log"
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

	sb := strings.Builder{}
	Hello(&sb)
	cliCtx.Printf("%s", sb.String())
}

func Hello(r io.Writer) {
	_, err := r.Write([]byte("Hello World!"))
	if err != nil {
		panic(err)
	}
}
