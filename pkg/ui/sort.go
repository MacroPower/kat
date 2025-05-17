package ui

import (
	"cmp"
	"slices"
)

func sortYAMLs(mds []*yaml) {
	slices.SortStableFunc(mds, func(a, b *yaml) int {
		return cmp.Compare(a.Note, b.Note)
	})
}
