package stash

import (
	"cmp"
	"slices"

	"github.com/MacroPower/kat/pkg/ui/yamldoc"
)

func sortYAMLs(mds []*yamldoc.YAMLDocument) {
	slices.SortStableFunc(mds, func(a, b *yamldoc.YAMLDocument) int {
		return cmp.Compare(a.Title, b.Title)
	})
}
