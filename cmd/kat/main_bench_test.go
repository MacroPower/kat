package main_test

import (
	"strings"
	"testing"

	main "github.com/MacroPower/kat/cmd/kat"
)

func BenchmarkMain(b *testing.B) {
	for range b.N {
		sb := strings.Builder{}
		main.Hello(&sb)
		sb.Reset()
	}
}
