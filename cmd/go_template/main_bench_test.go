package main_test

import (
	"strings"
	"testing"

	main "github.com/MacroPower/go_template/cmd/go_template"
)

func BenchmarkMain(b *testing.B) {
	for range b.N {
		sb := strings.Builder{}
		main.Hello(&sb)
		sb.Reset()
	}
}
