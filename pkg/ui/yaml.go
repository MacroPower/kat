package ui

import (
	"fmt"
	"unicode"

	"github.com/charmbracelet/log"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type yaml struct {
	object *unstructured.Unstructured

	// Value we filter against. This exists so that we can maintain positions
	// of filtered items if titles are edited while a filter is active. This
	// field is ephemeral, and should only be referenced during filtering.
	filterValue string

	Body  string
	Title string
	Desc  string
}

// Generate the value we're doing to filter against.
func (m *yaml) buildFilterValue() {
	title, err := normalize(m.Title)
	if err != nil {
		log.Error("error normalizing", "title", m.Title, "error", err)
		m.filterValue = m.Title
	}

	m.filterValue = title
}

// Normalize text to aid in the filtering process. In particular, we remove
// diacritics, "ö" becomes "o". Title that Mn is the unicode key for nonspacing
// marks.
func normalize(in string) (string, error) {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, in)
	if err != nil {
		return "", fmt.Errorf("error normalizing: %w", err)
	}

	return out, nil
}
