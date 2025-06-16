package keys

import (
	"fmt"
	"strings"

	"github.com/muesli/reflow/ansi"

	"github.com/MacroPower/kat/pkg/ui/themes"
)

type Key struct {
	Code   string `json:"code"             yaml:"code"`
	Alias  string `json:"alias,omitempty"  yaml:"alias,omitempty"`
	Hidden bool   `json:"hidden,omitempty" yaml:"hidden,omitempty"`
}

type KeyOpt func(k *Key)

func New(code string, opts ...KeyOpt) Key {
	k := &Key{
		Code: code,
	}
	for _, opt := range opts {
		opt(k)
	}

	return *k
}

func WithAlias(alias string) KeyOpt {
	return func(k *Key) {
		k.Alias = alias
	}
}

func Hidden() KeyOpt {
	return func(k *Key) {
		k.Hidden = true
	}
}

func (k Key) String() string {
	if k.Alias != "" {
		return k.Alias
	}

	return k.Code
}

type KeyBind struct {
	Description string
	Keys        []Key
}

func NewBind(description string, keys ...Key) KeyBind {
	return KeyBind{
		Description: description,
		Keys:        keys,
	}
}

func (kb *KeyBind) String() string {
	keys := []string{}
	for _, k := range kb.Keys {
		if k.Hidden {
			continue
		}
		keys = append(keys, k.String())
	}

	return strings.Join(keys, "/")
}

// KeyWidth should generally be the maximum width of any individual keybind
// string in the column.
func (kb *KeyBind) StringRow(keyWidth, descWidth int) string {
	keys := kb.String()
	if keys == "" {
		return "" // No keybinds or all keybinds are hidden.
	}

	truncDesc := truncateWithEllipsis(kb.Description, descWidth-2)

	keySpaces := strings.Repeat(" ", max(0, keyWidth-ansi.PrintableRuneWidth(keys)))
	descSpaces := strings.Repeat(" ", max(0, descWidth-ansi.PrintableRuneWidth(truncDesc)-2))

	return fmt.Sprintf("%s%s  %s%s", keys, keySpaces, truncDesc, descSpaces)
}

func (kb *KeyBind) Match(key string) bool {
	for _, k := range kb.Keys {
		if k.Code == key {
			return true
		}
	}

	return false
}

func (kb *KeyBind) AddKey(key Key) {
	if kb == nil {
		return
	}

	for _, k := range kb.Keys {
		if k.Code == key.Code {
			return // Key already exists, do not add again.
		}
	}

	kb.Keys = append(kb.Keys, key)
}

type KeyBindRenderer struct {
	columns [][]KeyBind
}

func (kbr *KeyBindRenderer) AddColumn(kbs ...KeyBind) {
	if len(kbs) == 0 {
		return
	}

	if kbr.columns == nil {
		kbr.columns = [][]KeyBind{}
	}

	kbr.columns = append(kbr.columns, kbs)
}

func (kbr *KeyBindRenderer) Render(width int) string {
	numCols := len(kbr.columns)
	if numCols == 0 {
		return "" // No columns to render.
	}

	colWidth := width
	colRemainder := 0

	if numCols > 1 {
		colWidth = width / numCols
		colRemainder = width % numCols
	}

	colWidth = max(6, colWidth-2)
	colRemainder = max(0, colRemainder)

	// Convert each column to an array of row strings.
	colRows := make([][]string, numCols)
	maxRows := 0

	for i, col := range kbr.columns {
		colOutput := stringColumn(colWidth, col...)
		colRows[i] = append(colRows[i], colOutput...)
		if len(colRows[i]) > maxRows {
			maxRows = len(colRows[i])
		}
	}

	// Build the final output by combining rows from all columns.
	var sb strings.Builder
	for row := range maxRows {
		for col := range colRows {
			// Get the row content for this column, or empty string if column doesn't have this row.
			var rowContent string
			if row < len(colRows[col]) {
				rowContent = colRows[col][row]
			} else {
				// Pad with spaces to match column width.
				rowContent = strings.Repeat(" ", colWidth)
			}
			sb.WriteString(" " + rowContent + " ")
		}

		// Add remainder spaces.
		sb.WriteString(strings.Repeat(" ", colRemainder))

		// Add a newline after each row except the last one.
		if row < maxRows-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func stringColumn(width int, kbs ...KeyBind) []string {
	if len(kbs) == 0 {
		return []string{} // No keybinds to render.
	}

	// Get the maximum width for the keybinds.
	maxKeyWidth := 0
	for _, kb := range kbs {
		chars := ansi.PrintableRuneWidth(kb.String())
		if chars > maxKeyWidth {
			maxKeyWidth = chars
		}
	}

	rows := []string{}
	for _, kb := range kbs {
		row := kb.StringRow(maxKeyWidth, width-maxKeyWidth)
		if row != "" {
			rows = append(rows, row)
		}
	}

	return rows
}

func ValidateBinds(kbs ...[]KeyBind) error {
	seen := make(map[string]bool)
	for _, ks := range kbs {
		for _, kb := range ks {
			for _, key := range kb.Keys {
				if seen[key.Code] {
					return fmt.Errorf("duplicate key binding found: %s", key.Code)
				}
				seen[key.Code] = true
			}
		}
	}

	return nil
}

func SetDefaultBind(kb **KeyBind, defaultKb KeyBind) {
	if *kb == nil {
		*kb = &defaultKb

		return
	}

	if len((*kb).Keys) == 0 {
		(*kb).Keys = defaultKb.Keys
	}

	if (*kb).Description == "" {
		(*kb).Description = defaultKb.Description
	}
}

// truncateWithEllipsis truncates a string with ellipsis if it exceeds maxWidth.
func truncateWithEllipsis(s string, maxWidth int) string {
	if maxWidth <= 0 {
		if s == "" {
			return ""
		}

		return themes.Ellipsis
	}
	if ansi.PrintableRuneWidth(s) <= maxWidth {
		return s
	}

	lenEllipsis := ansi.PrintableRuneWidth(themes.Ellipsis)

	// Reserve space for ellipsis.
	if maxWidth <= lenEllipsis {
		return themes.Ellipsis[:maxWidth]
	}

	// Simple truncation - could be improved with proper text handling.
	availableWidth := maxWidth - lenEllipsis
	truncated := ""
	currentWidth := 0

	for _, r := range s {
		runeWidth := ansi.PrintableRuneWidth(string(r))
		if currentWidth+runeWidth > availableWidth {
			break
		}
		truncated += string(r)
		currentWidth += runeWidth
	}

	return truncated + themes.Ellipsis
}
