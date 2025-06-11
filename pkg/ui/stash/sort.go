package stash

import (
	"slices"
	"strings"

	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

func sortYAMLs(mds []*yamldoc.YAMLDocument) {
	slices.SortStableFunc(mds, func(a, b *yamldoc.YAMLDocument) int {
		return strings.Compare(strings.ToLower(a.Desc+a.Title), strings.ToLower(b.Desc+b.Title))
	})
}
