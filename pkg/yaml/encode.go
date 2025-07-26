package yaml

import (
	"io"

	"github.com/goccy/go-yaml"
)

type Encoder struct {
	e *yaml.Encoder
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		e: yaml.NewEncoder(w, yaml.Indent(2), yaml.IndentSequence(true)),
	}
}

func (e *Encoder) Encode(v any) error {
	return e.e.Encode(v) //nolint:wrapcheck // Return the original error.
}

func (e *Encoder) Close() error {
	return e.e.Close() //nolint:wrapcheck // Return the original error.
}
