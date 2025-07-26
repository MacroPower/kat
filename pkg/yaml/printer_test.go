package yaml_test

import (
	"testing"

	"github.com/goccy/go-yaml/lexer"
	"github.com/stretchr/testify/assert"

	"github.com/macropower/kat/pkg/yaml"
)

func Test_PrinterErrorToken(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input      string
		want       string
		tokenIndex int
		wantLine   int
	}{
		"basic yaml tokens[3]": {
			input: `---
text: aaaa
text2: aaaa
 bbbb
 cccc
 dddd
 eeee
text3: ffff
 gggg
 hhhh
 iiii
 jjjj
bool: true
number: 10
anchor: &x 1
alias: *x
`,
			tokenIndex: 3,
			want: `
---
text: aaaa
text2: aaaa
 bbbb
 cccc
 dddd
 eeee
`,
			wantLine: 1,
		},
		"basic yaml tokens[4]": {
			input: `---
text: aaaa
text2: aaaa
 bbbb
 cccc
 dddd
 eeee
text3: ffff
 gggg
 hhhh
 iiii
 jjjj
bool: true
number: 10
anchor: &x 1
alias: *x
`,
			tokenIndex: 4,
			want: `
---
text: aaaa
text2: aaaa
 bbbb
 cccc
 dddd
 eeee
`,
			wantLine: 1,
		},
		"basic yaml tokens[6]": {
			input: `---
text: aaaa
text2: aaaa
 bbbb
 cccc
 dddd
 eeee
text3: ffff
 gggg
 hhhh
 iiii
 jjjj
bool: true
number: 10
anchor: &x 1
alias: *x
`,
			tokenIndex: 6,
			want: `
---
text: aaaa
text2: aaaa
 bbbb
 cccc
 dddd
 eeee
text3: ffff
 gggg
 hhhh
 iiii
 jjjj
`,
			wantLine: 1,
		},
		"document header tokens[12]": {
			input: `---
a:
 b:
  c:
   d: e
   f: g
   h: i

---
`,
			tokenIndex: 12,
			want: `
 b:
  c:
   d: e
   f: g
   h: i

---`,
			wantLine: 3,
		},
		"multiline strings tokens[2]": {
			input: `
text1: 'aaaa
 bbbb
 cccc'
text2: "ffff
 gggg
 hhhh"
text3: hello
`,
			tokenIndex: 2,
			want: `
text1: 'aaaa
 bbbb
 cccc'
text2: "ffff
 gggg
 hhhh"`,
			wantLine: 1,
		},
		"multiline strings tokens[3]": {
			input: `
text1: 'aaaa
 bbbb
 cccc'
text2: "ffff
 gggg
 hhhh"
text3: hello
`,
			tokenIndex: 3,
			want: `
text1: 'aaaa
 bbbb
 cccc'
text2: "ffff
 gggg
 hhhh"
text3: hello`,
			wantLine: 2,
		},
		"multiline strings tokens[5]": {
			input: `
text1: 'aaaa
 bbbb
 cccc'
text2: "ffff
 gggg
 hhhh"
text3: hello
`,
			tokenIndex: 5,
			want: `
text1: 'aaaa
 bbbb
 cccc'
text2: "ffff
 gggg
 hhhh"
text3: hello`,
			wantLine: 2,
		},
		"multiline strings tokens[6]": {
			input: `
text1: 'aaaa
 bbbb
 cccc'
text2: "ffff
 gggg
 hhhh"
text3: hello
`,
			tokenIndex: 6,
			want: `
text2: "ffff
 gggg
 hhhh"
text3: hello
`,
			wantLine: 5,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tokens := lexer.Tokenize(tc.input)

			var p yaml.Printer

			got, gotLine := p.PrintErrorToken(tokens[tc.tokenIndex], 3)
			got = "\n" + got

			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantLine, gotLine)
		})
	}
}

func TestPrinter_Anchor(t *testing.T) {
	t.Parallel()

	input := `
anchor: &x 1
alias: *x`
	tokens := lexer.Tokenize(input)

	var p yaml.Printer

	got := p.PrintTokens(tokens)

	assert.Equal(t, input, got)
}
