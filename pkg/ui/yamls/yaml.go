package yamls

import (
	"fmt"
	"log/slog"
	"unicode"

	"go.jacobcolvin.com/niceyaml"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/macropower/kat/pkg/kube"
)

type Document struct {
	Object *kube.Object

	// FilterValue is the value we filter against. This exists so that we can
	// maintain positions of filtered items if titles are edited while a filter
	// is active. This field is ephemeral, and should only be referenced during
	// filtering. Use the FilterValue() method to access it.
	filterValue string

	Body  *niceyaml.Source
	Title string
	Desc  string
}

// FilterValue returns the value to filter against.
// This method allows Document to satisfy list.Item.
func (m *Document) FilterValue() string {
	return m.filterValue
}

// BuildFilterValue generates the filter value from Title and Desc.
func (m *Document) BuildFilterValue() {
	m.filterValue = ""

	title, err := Normalize(m.Title)
	if err == nil {
		m.filterValue += title
	} else {
		slog.Error("error normalizing",
			slog.String("title", m.Title),
			slog.Any("error", err),
		)

		m.filterValue += m.Title
	}

	desc, err := Normalize(m.Desc)
	if err == nil {
		m.filterValue += desc
	} else {
		slog.Error("error normalizing",
			slog.String("desc", m.Desc),
			slog.Any("error", err),
		)

		m.filterValue += m.Desc
	}
}

// Normalize text to aid in the filtering process. In particular, we remove
// diacritics, "ö" becomes "o". Title that Mn is the unicode key for nonspacing
// marks.
func Normalize(in string) (string, error) {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, in)
	if err != nil {
		return "", fmt.Errorf("error normalizing: %w", err)
	}

	return out, nil
}
