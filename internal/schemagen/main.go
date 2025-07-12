package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"github.com/macropower/kat/pkg/config"
	"github.com/macropower/kat/pkg/schema"
)

var cli struct {
	OutFile string `default:"schema.json" help:"Output file for the generated schema" short:"o"`
}

func main() {
	cliCtx := kong.Parse(&cli)

	gen := schema.NewGenerator(config.NewConfig(),
		"github.com/macropower/kat/pkg/config",
		"github.com/macropower/kat/pkg/command",
		"github.com/macropower/kat/pkg/ui",
		"github.com/macropower/kat/pkg/profile",
		"github.com/macropower/kat/pkg/rule",
		"github.com/macropower/kat/pkg/execs",
		"github.com/macropower/kat/pkg/keys",
	)
	jsData, err := gen.Generate()
	if err != nil {
		cliCtx.FatalIfErrorf(fmt.Errorf("generate JSON schema: %w", err))
	}

	// Write schema.json file.
	err = os.WriteFile(cli.OutFile, jsData, 0o600)
	if err != nil {
		cliCtx.FatalIfErrorf(fmt.Errorf("write schema file: %w", err))
	}
}
