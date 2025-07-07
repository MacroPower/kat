package list

import (
	"slices"
	"strings"

	"github.com/macropower/kat/pkg/ui/yamls"
)

func sortYAMLs(mds []*yamls.Document) {
	slices.SortStableFunc(mds, func(a, b *yamls.Document) int {
		return strings.Compare(strings.ToLower(a.Desc+a.Title), strings.ToLower(b.Desc+b.Title))
	})
}
