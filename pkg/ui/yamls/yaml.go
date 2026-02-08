package yamls

import (
	"go.jacobcolvin.com/niceyaml"
	"go.jacobcolvin.com/niceyaml/normalizer"

	"github.com/macropower/kat/pkg/kube"
)

var norm = normalizer.New(normalizer.WithCaseFold(false))

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
	m.filterValue = Normalize(m.Title) + Normalize(m.Desc)
}

// Normalize removes diacritics from text to aid in the filtering process.
// For example, "ö" becomes "o".
func Normalize(in string) string {
	return norm.Normalize(in)
}
