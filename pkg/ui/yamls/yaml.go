package yamls

import (
	"log/slog"

	"github.com/macropower/kat/pkg/kube"
)

type Document struct {
	Object *kube.Object

	// Value we filter against. This exists so that we can maintain positions
	// of filtered items if titles are edited while a filter is active. This
	// field is ephemeral, and should only be referenced during filtering.
	FilterValue string

	Body  string
	Title string
	Desc  string
}

// Generate the value we're doing to filter against.
func (m *Document) BuildFilterValue() {
	m.FilterValue = ""

	title, err := Normalize(m.Title)
	if err == nil {
		m.FilterValue += title
	} else {
		slog.Error("error normalizing",
			slog.String("title", m.Title),
			slog.Any("error", err),
		)

		m.FilterValue += m.Title
	}

	desc, err := Normalize(m.Desc)
	if err == nil {
		m.FilterValue += desc
	} else {
		slog.Error("error normalizing",
			slog.String("desc", m.Desc),
			slog.Any("error", err),
		)

		m.FilterValue += m.Desc
	}
}
