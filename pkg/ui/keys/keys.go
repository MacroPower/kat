package keys

// KeyBindings holds the key binding mappings for different contexts.
type KeyBindings struct {
	Quit     []string
	Suspend  []string
	Escape   []string
	Refresh  []string
	Navigate NavigationKeys
	Document DocumentKeys
	Filter   FilterKeys
}

type NavigationKeys struct {
	Up        []string
	Down      []string
	Left      []string
	Right     []string
	Home      []string
	End       []string
	PageUp    []string
	PageDown  []string
	PageLeft  []string
	PageRight []string
}

type DocumentKeys struct {
	Open  []string
	Close []string
	Copy  []string
}

type FilterKeys struct {
	Start  []string
	Apply  []string
	Clear  []string
	Cancel []string
}

var DefaultKeyBindings = KeyBindings{
	Quit:    []string{"q", "ctrl+c"},
	Suspend: []string{"ctrl+z"},
	Escape:  []string{"esc"},
	Refresh: []string{"r"},
	Navigate: NavigationKeys{
		Up:        []string{"k", "up"},
		Down:      []string{"j", "down"},
		Left:      []string{"left", "h"},
		Right:     []string{"right", "l"},
		Home:      []string{"home", "g"},
		End:       []string{"end", "G"},
		PageUp:    []string{"b", "u"},
		PageDown:  []string{"f", "d"},
		PageLeft:  []string{"shift+tab", "H"},
		PageRight: []string{"tab", "L"},
	},
	Document: DocumentKeys{
		Open:  []string{"enter"},
		Close: []string{"q", "esc"},
		Copy:  []string{"c"},
	},
	Filter: FilterKeys{
		Start:  []string{"/"},
		Apply:  []string{"enter", "tab"},
		Clear:  []string{"esc"},
		Cancel: []string{"esc"},
	},
}

// KeyMatches checks if a key string matches any of the provided patterns.
func KeyMatches(key string, patterns []string) bool {
	for _, pattern := range patterns {
		if key == pattern {
			return true
		}
	}

	return false
}
