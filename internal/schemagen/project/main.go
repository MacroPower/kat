package main

import (
	"flag"
	"log"
	"os"

	"go.jacobcolvin.com/niceyaml/schema/generator"

	"github.com/macropower/kat/api/v1beta1/runtimeconfigs"
)

var outFile = flag.String("o", "schema.json", "Output file for the generated schema")

func main() {
	flag.Parse()

	gen := generator.New(runtimeconfigs.New(),
		generator.WithPackagePaths(
			"github.com/macropower/kat/api/v1beta1",
			"github.com/macropower/kat/api/v1beta1/runtimeconfigs",
			"github.com/macropower/kat/pkg/command",
			"github.com/macropower/kat/pkg/execs",
			"github.com/macropower/kat/pkg/profile",
			"github.com/macropower/kat/pkg/rule",
		),
	)
	jsData, err := gen.Generate()
	if err != nil {
		log.Fatalf("generate JSON schema: %v", err)
	}

	// Write schema.json file.
	err = os.WriteFile(*outFile, jsData, 0o600)
	if err != nil {
		log.Fatalf("write schema file: %v", err)
	}
}
