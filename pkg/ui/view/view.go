package view

import (
	"strings"
)

func AlwaysPlaceBottom(content string) string {
	return strings.TrimRight(content, " \n")
}
