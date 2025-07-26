package yaml

import (
	"errors"
	"io"

	"github.com/goccy/go-yaml"
)

type Decoder struct {
	d *yaml.Decoder
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		d: yaml.NewDecoder(r, yaml.AllowDuplicateMapKey()),
	}
}

func (d *Decoder) Decode(v any) error {
	err := d.d.Decode(v)
	if err == nil {
		return nil
	}

	var yamlErr yaml.Error
	if errors.As(err, &yamlErr) {
		return &Error{
			Err:   errors.New(yamlErr.GetMessage()),
			Token: yamlErr.GetToken(),
		}
	}

	//nolint:wrapcheck // Return the original error if it's not a [yaml.Error].
	return err
}
