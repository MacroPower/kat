package yaml

import (
	"fmt"
	"math"
	"strings"

	"github.com/goccy/go-yaml/token"
)

// Taken from https://github.com/goccy/go-yaml
// MIT License.
// Copyright (c) 2019 Masaaki Goshima.

// Property additional property set for each the token.
type Property struct {
	Prefix string
	Suffix string
}

// PrintFunc returns property instance.
type PrintFunc func() *Property

// Printer create text from token collection or ast.
// This is a simplified version of [github.com/goccy/go-yaml/printer.Printer],
// with styling and annotation functionality removed.
type Printer struct {
	MapKey  PrintFunc
	Anchor  PrintFunc
	Alias   PrintFunc
	Bool    PrintFunc
	String  PrintFunc
	Number  PrintFunc
	Comment PrintFunc
}

func (p *Printer) property(tk *token.Token) *Property {
	prop := &Property{}

	//nolint:exhaustive // Only needed for the current token.
	switch tk.PreviousType() {
	case token.AnchorType:
		if p.Anchor != nil {
			return p.Anchor()
		}

		return prop

	case token.AliasType:
		if p.Alias != nil {
			return p.Alias()
		}

		return prop
	}

	//nolint:exhaustive // Only needed for the current token.
	switch tk.NextType() {
	case token.MappingValueType:
		if p.MapKey != nil {
			return p.MapKey()
		}

		return prop
	}

	switch tk.Type {
	case token.BoolType:
		if p.Bool != nil {
			return p.Bool()
		}

		return prop

	case token.AnchorType:
		if p.Anchor != nil {
			return p.Anchor()
		}

		return prop

	case token.AliasType:
		if p.Anchor != nil {
			return p.Alias()
		}

		return prop

	case token.StringType, token.SingleQuoteType, token.DoubleQuoteType:
		if p.String != nil {
			return p.String()
		}

		return prop

	case token.IntegerType, token.FloatType:
		if p.Number != nil {
			return p.Number()
		}

		return prop

	case token.CommentType:
		if p.Comment != nil {
			return p.Comment()
		}

		return prop

	default:
		return prop
	}
}

// PrintTokens create text from token collection.
func (p *Printer) PrintTokens(tokens token.Tokens) string {
	if len(tokens) == 0 {
		return ""
	}

	texts := []string{}
	for _, tk := range tokens {
		lines := strings.Split(tk.Origin, "\n")
		prop := p.property(tk)
		header := ""

		if len(lines) == 1 {
			line := prop.Prefix + lines[0] + prop.Suffix
			if len(texts) == 0 {
				texts = append(texts, header+line)
			} else {
				text := texts[len(texts)-1]
				texts[len(texts)-1] = text + line
			}

			continue
		}

		for idx, src := range lines {
			line := prop.Prefix + src + prop.Suffix
			if idx == 0 {
				if len(texts) == 0 {
					texts = append(texts, header+line)
				} else {
					text := texts[len(texts)-1]
					texts[len(texts)-1] = text + line
				}
			} else {
				texts = append(texts, fmt.Sprintf("%s%s", header, line))
			}
		}
	}

	return strings.Join(texts, "\n")
}

func (p *Printer) removeLeftSideNewLineChar(src string) string {
	return strings.TrimLeft(strings.TrimLeft(strings.TrimLeft(src, "\r"), "\n"), "\r\n")
}

func (p *Printer) removeRightSideNewLineChar(src string) string {
	return strings.TrimRight(strings.TrimRight(strings.TrimRight(src, "\r"), "\n"), "\r\n")
}

func (p *Printer) removeRightSideWhiteSpaceChar(src string) string {
	return p.removeRightSideNewLineChar(strings.TrimRight(src, " "))
}

func (p *Printer) newLineCount(s string) int {
	src := []rune(s)
	size := len(src)

	cnt := 0
	for i := 0; i < size; i++ {
		c := src[i]
		switch c {
		case '\r':
			if i+1 < size && src[i+1] == '\n' {
				i++
			}

			cnt++

		case '\n':
			cnt++
		}
	}

	return cnt
}

func (p *Printer) isNewLineLastChar(s string) bool {
	for i := len(s) - 1; i > 0; i-- {
		c := s[i]
		switch c {
		case ' ':
			continue
		case '\n', '\r':
			return true
		}

		break
	}

	return false
}

func (p *Printer) printBeforeTokens(tk *token.Token, minLine, extLine int) token.Tokens {
	for tk.Prev != nil {
		if tk.Prev.Position.Line < minLine {
			break
		}

		tk = tk.Prev
	}

	minTk := tk.Clone()
	if minTk.Prev != nil {
		// Add white spaces to minTk by prev token.
		prev := minTk.Prev
		whiteSpaceLen := len(prev.Origin) - len(strings.TrimRight(prev.Origin, " "))
		minTk.Origin = strings.Repeat(" ", whiteSpaceLen) + minTk.Origin
	}

	minTk.Origin = p.removeLeftSideNewLineChar(minTk.Origin)
	tokens := token.Tokens{minTk}

	tk = minTk.Next
	for tk != nil && tk.Position.Line <= extLine {
		clonedTk := tk.Clone()
		tokens.Add(clonedTk)

		tk = clonedTk.Next
	}

	lastTk := tokens[len(tokens)-1]
	trimmedOrigin := p.removeRightSideWhiteSpaceChar(lastTk.Origin)
	suffix := lastTk.Origin[len(trimmedOrigin):]
	lastTk.Origin = trimmedOrigin

	if lastTk.Next != nil && len(suffix) > 1 {
		next := lastTk.Next.Clone()
		// Add suffix to header of next token.
		if suffix[0] == '\n' || suffix[0] == '\r' {
			suffix = suffix[1:]
		}

		next.Origin = suffix + next.Origin
		lastTk.Next = next
	}

	return tokens
}

func (p *Printer) printAfterTokens(tk *token.Token, maxLine int) token.Tokens {
	tokens := token.Tokens{}
	if tk == nil {
		return tokens
	}
	if tk.Position.Line > maxLine {
		return tokens
	}

	minTk := tk.Clone()
	minTk.Origin = p.removeLeftSideNewLineChar(minTk.Origin)
	tokens.Add(minTk)

	tk = minTk.Next
	for tk != nil && tk.Position.Line <= maxLine {
		clonedTk := tk.Clone()
		tokens.Add(clonedTk)

		tk = clonedTk.Next
	}

	return tokens
}

func (p *Printer) PrintErrorToken(tk *token.Token, lines int) (string, int) {
	curLine := tk.Position.Line
	curExtLine := curLine + p.newLineCount(p.removeLeftSideNewLineChar(tk.Origin))
	if p.isNewLineLastChar(tk.Origin) {
		// If last character ( exclude white space ) is new line character, ignore it.
		curExtLine--
	}

	minLine := int(math.Max(float64(curLine-lines), 1))
	maxLine := curExtLine + lines

	beforeTokens := p.printBeforeTokens(tk, minLine, curExtLine)
	lastTk := beforeTokens[len(beforeTokens)-1]
	afterTokens := p.printAfterTokens(lastTk.Next, maxLine)

	beforeSource := p.PrintTokens(beforeTokens)
	afterSource := p.PrintTokens(afterTokens)

	return fmt.Sprintf("%s\n%s", beforeSource, afterSource), minLine
}
