package main_test

import (
	"strings"
	"testing"

	main "github.com/MacroPower/go_template/cmd/go_template"
)

func TestMain(t *testing.T) {
	t.Parallel()

	sb := strings.Builder{}
	main.Hello(&sb)

	if want := "Hello World!"; sb.String() != want {
		t.Fatalf("expected %s, got %s", want, sb.String())
	}
}
