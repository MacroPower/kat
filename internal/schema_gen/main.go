package main

import (
	"fmt"
	"os"

	"github.com/macropower/kat/pkg/config"
	"github.com/macropower/kat/pkg/schema"
)

func main() {
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
		panic(fmt.Errorf("generate JSON schema: %w", err))
	}

	// Write schema.json file.
	if err := os.WriteFile("schema.json", jsData, 0o600); err != nil {
		panic(fmt.Errorf("write schema file: %w", err))
	}
}
