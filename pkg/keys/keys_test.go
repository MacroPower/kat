package keys_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/keys"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		code     string
		opts     []keys.KeyOpt
		expected keys.Key
	}{
		"basic key creation": {
			code: "ctrl+c",
			expected: keys.Key{
				Code:   "ctrl+c",
				Alias:  "",
				Hidden: false,
			},
		},
		"key with alias": {
			code: "ctrl+c",
			opts: []keys.KeyOpt{keys.WithAlias("⌃c")},
			expected: keys.Key{
				Code:   "ctrl+c",
				Alias:  "⌃c",
				Hidden: false,
			},
		},
		"hidden key": {
			code: "esc",
			opts: []keys.KeyOpt{keys.Hidden()},
			expected: keys.Key{
				Code:   "esc",
				Alias:  "",
				Hidden: true,
			},
		},
		"key with both alias and hidden": {
			code: "ctrl+z",
			opts: []keys.KeyOpt{keys.WithAlias("⌃z"), keys.Hidden()},
			expected: keys.Key{
				Code:   "ctrl+z",
				Alias:  "⌃z",
				Hidden: true,
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := keys.New(tc.code, tc.opts...)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestWithAlias(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		alias    string
		expected string
	}{
		"simple alias": {
			alias:    "⌃c",
			expected: "⌃c",
		},
		"empty alias": {
			alias:    "",
			expected: "",
		},
		"unicode alias": {
			alias:    "↑",
			expected: "↑",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			key := keys.New("test", keys.WithAlias(tc.alias))
			assert.Equal(t, tc.expected, key.Alias)
		})
	}
}

func TestHidden(t *testing.T) {
	t.Parallel()

	key := keys.New("test", keys.Hidden())
	assert.True(t, key.Hidden)
}

func TestKey_String(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		expected string
		key      keys.Key
	}{
		"key with alias returns alias": {
			key: keys.Key{
				Code:  "ctrl+c",
				Alias: "⌃c",
			},
			expected: "⌃c",
		},
		"key without alias returns code": {
			key: keys.Key{
				Code: "ctrl+c",
			},
			expected: "ctrl+c",
		},
		"key with empty alias returns code": {
			key: keys.Key{
				Code:  "enter",
				Alias: "",
			},
			expected: "enter",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := tc.key.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNewBind(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		description string
		keys        []keys.Key
		expected    keys.KeyBind
	}{
		"bind with single key": {
			description: "quit",
			keys:        []keys.Key{keys.New("q")},
			expected: keys.KeyBind{
				Description: "quit",
				Keys:        []keys.Key{keys.New("q")},
			},
		},
		"bind with multiple keys": {
			description: "move up",
			keys:        []keys.Key{keys.New("k"), keys.New("up", keys.WithAlias("↑"))},
			expected: keys.KeyBind{
				Description: "move up",
				Keys:        []keys.Key{keys.New("k"), keys.New("up", keys.WithAlias("↑"))},
			},
		},
		"bind with no keys": {
			description: "empty",
			keys:        []keys.Key{},
			expected: keys.KeyBind{
				Description: "empty",
				Keys:        []keys.Key{},
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := keys.NewBind(tc.description, tc.keys...)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestKeyBind_String(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		expected string
		keyBind  keys.KeyBind
	}{
		"single key": {
			keyBind: keys.KeyBind{
				Description: "quit",
				Keys:        []keys.Key{keys.New("q")},
			},
			expected: "q",
		},
		"multiple keys": {
			keyBind: keys.KeyBind{
				Description: "move up",
				Keys:        []keys.Key{keys.New("k"), keys.New("up", keys.WithAlias("↑"))},
			},
			expected: "k/↑",
		},
		"keys with some hidden": {
			keyBind: keys.KeyBind{
				Description: "quit",
				Keys:        []keys.Key{keys.New("q"), keys.New("ctrl+c", keys.Hidden())},
			},
			expected: "q",
		},
		"all keys hidden": {
			keyBind: keys.KeyBind{
				Description: "hidden action",
				Keys:        []keys.Key{keys.New("ctrl+x", keys.Hidden()), keys.New("ctrl+y", keys.Hidden())},
			},
			expected: "",
		},
		"no keys": {
			keyBind: keys.KeyBind{
				Description: "empty",
				Keys:        []keys.Key{},
			},
			expected: "",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := tc.keyBind.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestKeyBind_StringRow(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		expected  string
		keyBind   keys.KeyBind
		keyWidth  int
		descWidth int
	}{
		"pad key and description": {
			keyBind: keys.KeyBind{
				Description: "quit",
				Keys:        []keys.Key{keys.New("q")},
			},
			keyWidth:  5,
			descWidth: 10,
			expected:  "q      quit    ",
		},
		"truncate long descriptions with ellipsis": {
			keyBind: keys.KeyBind{
				Description: "very long description that should be truncated",
				Keys:        []keys.Key{keys.New("q")},
			},
			keyWidth:  5,
			descWidth: 10,
			expected:  "q      very lo…",
		},
		"empty keybind": {
			keyBind: keys.KeyBind{
				Description: "hidden",
				Keys:        []keys.Key{keys.New("ctrl+x", keys.Hidden())},
			},
			keyWidth:  5,
			descWidth: 10,
			expected:  "",
		},
		"zero widths": {
			keyBind: keys.KeyBind{
				Description: "test",
				Keys:        []keys.Key{keys.New("q")},
			},
			keyWidth:  0,
			descWidth: 0,
			expected:  "q  …",
		},
		"key longer than width": {
			keyBind: keys.KeyBind{
				Description: "test",
				Keys:        []keys.Key{keys.New("ctrl+shift+alt+a")},
			},
			keyWidth:  5,
			descWidth: 10,
			expected:  "ctrl+shift+alt+a  test    ",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := tc.keyBind.StringRow(tc.keyWidth, tc.descWidth)
			assert.Equal(t, tc.expected, result, "for keybind: %v, keyWidth: %d, descWidth: %d",
				tc.keyBind, tc.keyWidth, tc.descWidth)
		})
	}
}

func TestKeyBind_Match(t *testing.T) {
	t.Parallel()

	keyBind := keys.KeyBind{
		Description: "move up",
		Keys:        []keys.Key{keys.New("k"), keys.New("up", keys.WithAlias("↑"))},
	}

	tcs := map[string]struct {
		input    string
		expected bool
	}{
		"matches first key": {
			input:    "k",
			expected: true,
		},
		"matches second key code": {
			input:    "up",
			expected: true,
		},
		"does not match alias": {
			input:    "↑",
			expected: false,
		},
		"does not match non-existent key": {
			input:    "j",
			expected: false,
		},
		"empty input": {
			input:    "",
			expected: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := keyBind.Match(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestKeyBind_IsInputAction(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input string
		want  bool
	}{
		"accepts single letter": {
			input: "k",
			want:  true,
		},
		"accepts uppercase letter": {
			input: "K",
			want:  true,
		},
		"rejects navigation key": {
			input: "up",
			want:  false,
		},
		"accepts copy": {
			input: "ctrl+c",
			want:  true,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := keys.IsTextInputAction(tc.input)
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestKeyBind_AddKey(t *testing.T) {
	t.Parallel()

	t.Run("add key to non-nil keybind", func(t *testing.T) {
		t.Parallel()

		kb := &keys.KeyBind{
			Description: "test",
			Keys:        []keys.Key{keys.New("a")},
		}

		newKey := keys.New("b")
		kb.AddKey(newKey)

		assert.Len(t, kb.Keys, 2)
		assert.Equal(t, "a", kb.Keys[0].Code)
		assert.Equal(t, "b", kb.Keys[1].Code)
	})

	t.Run("add duplicate key", func(t *testing.T) {
		t.Parallel()

		kb := &keys.KeyBind{
			Description: "test",
			Keys:        []keys.Key{keys.New("a")},
		}

		duplicateKey := keys.New("a")
		kb.AddKey(duplicateKey)

		assert.Len(t, kb.Keys, 1, "should not add duplicate key")
		assert.Equal(t, "a", kb.Keys[0].Code)
	})

	t.Run("add key to nil keybind", func(t *testing.T) {
		t.Parallel()

		var kb *keys.KeyBind

		newKey := keys.New("a")

		// Should not panic
		assert.NotPanics(t, func() {
			kb.AddKey(newKey)
		})
	})

	t.Run("add key with different alias but same code", func(t *testing.T) {
		t.Parallel()

		kb := &keys.KeyBind{
			Description: "test",
			Keys:        []keys.Key{keys.New("ctrl+c", keys.WithAlias("⌃c"))},
		}

		duplicateKey := keys.New("ctrl+c", keys.WithAlias("Ctrl-C"))
		kb.AddKey(duplicateKey)

		assert.Len(t, kb.Keys, 1, "should not add key with same code even if alias differs")
	})
}

func TestStringColumn(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		expected string
		keyBinds []keys.KeyBind
		width    int
	}{
		"empty keybinds": {
			width:    20,
			keyBinds: []keys.KeyBind{},
			expected: "",
		},
		"single keybind": {
			width: 20,
			keyBinds: []keys.KeyBind{
				keys.NewBind("quit", keys.New("q")),
			},
			expected: " q  quit            ",
		},
		"multiple keybinds": {
			width: 25,
			keyBinds: []keys.KeyBind{
				keys.NewBind("quit", keys.New("q")),
				keys.NewBind("help", keys.New("?")),
				keys.NewBind("move up", keys.New("k"), keys.New("up", keys.WithAlias("↑"))),
			},
			expected: "" +
				" q    quit               \n" +
				" ?    help               \n" +
				" k/↑  move up            ",
		},
		"keybind with hidden keys": {
			width: 20,
			keyBinds: []keys.KeyBind{
				keys.NewBind("quit", keys.New("q")),
				keys.NewBind("hidden", keys.New("ctrl+x", keys.Hidden())),
				keys.NewBind("help", keys.New("?")),
			},
			expected: "" +
				" q  quit            \n" +
				" ?  help            ",
		},
		"zero width": {
			width: 0,
			keyBinds: []keys.KeyBind{
				keys.NewBind("test", keys.New("a")),
			},
			expected: " a  te… ",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			kbr := keys.KeyBindRenderer{}
			kbr.AddColumn(tc.keyBinds...)

			if tc.width > 0 {
				for line := range strings.Lines(tc.expected) {
					// Provided width should match expected line width.
					require.Equal(t, utf8.RuneCountInString(strings.TrimSuffix(line, "\n")), tc.width)
				}
			}

			result := kbr.Render(tc.width)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateBinds(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		errorContains string
		keyBindSets   [][]keys.KeyBind
		expectedError bool
	}{
		"no duplicate keys": {
			keyBindSets: [][]keys.KeyBind{
				{
					keys.NewBind("quit", keys.New("q")),
					keys.NewBind("help", keys.New("?")),
				},
				{
					keys.NewBind("up", keys.New("k")),
					keys.NewBind("down", keys.New("j")),
				},
			},
			expectedError: false,
		},
		"duplicate keys in same set": {
			keyBindSets: [][]keys.KeyBind{
				{
					keys.NewBind("quit", keys.New("q")),
					keys.NewBind("other", keys.New("q")),
				},
			},
			expectedError: true,
			errorContains: "duplicate key binding found: q",
		},
		"duplicate keys across sets": {
			keyBindSets: [][]keys.KeyBind{
				{
					keys.NewBind("quit", keys.New("q")),
				},
				{
					keys.NewBind("other", keys.New("q")),
				},
			},
			expectedError: true,
			errorContains: "duplicate key binding found: q",
		},
		"multiple keys in single bind": {
			keyBindSets: [][]keys.KeyBind{
				{
					keys.NewBind("move up", keys.New("k"), keys.New("up")),
					keys.NewBind("move down", keys.New("j"), keys.New("down")),
				},
			},
			expectedError: false,
		},
		"duplicate key within single keybind": {
			keyBindSets: [][]keys.KeyBind{
				{
					keys.NewBind("multi", keys.New("a"), keys.New("a")),
				},
			},
			expectedError: true,
			errorContains: "duplicate key binding found: a",
		},
		"empty sets": {
			keyBindSets:   [][]keys.KeyBind{},
			expectedError: false,
		},
		"empty keybinds": {
			keyBindSets: [][]keys.KeyBind{
				{},
				{},
			},
			expectedError: false,
		},
		"hidden keys can be duplicated": {
			keyBindSets: [][]keys.KeyBind{
				{
					keys.NewBind("quit", keys.New("q"), keys.New("ctrl+c", keys.Hidden())),
					keys.NewBind("other", keys.New("x"), keys.New("ctrl+c", keys.Hidden())),
				},
			},
			expectedError: true,
			errorContains: "duplicate key binding found: ctrl+c",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := keys.ValidateBinds(tc.keyBindSets...)

			if tc.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetDefaultBind(t *testing.T) {
	t.Parallel()

	t.Run("set default for nil keybind", func(t *testing.T) {
		t.Parallel()

		var kb *keys.KeyBind

		defaultKb := keys.NewBind("default", keys.New("d"))

		keys.SetDefaultBind(&kb, defaultKb)

		require.NotNil(t, kb)
		assert.Equal(t, "default", kb.Description)
		assert.Len(t, kb.Keys, 1)
		assert.Equal(t, "d", kb.Keys[0].Code)
	})

	t.Run("set keys for keybind with empty keys", func(t *testing.T) {
		t.Parallel()

		kb := &keys.KeyBind{
			Description: "existing",
			Keys:        []keys.Key{},
		}
		defaultKb := keys.NewBind("default", keys.New("d"))

		keys.SetDefaultBind(&kb, defaultKb)

		assert.Equal(t, "existing", kb.Description, "should keep existing description")
		assert.Len(t, kb.Keys, 1, "should set default keys")
		assert.Equal(t, "d", kb.Keys[0].Code)
	})

	t.Run("set description for keybind with empty description", func(t *testing.T) {
		t.Parallel()

		kb := &keys.KeyBind{
			Description: "",
			Keys:        []keys.Key{keys.New("existing")},
		}
		defaultKb := keys.NewBind("default", keys.New("d"))

		keys.SetDefaultBind(&kb, defaultKb)

		assert.Equal(t, "default", kb.Description, "should set default description")
		assert.Len(t, kb.Keys, 1, "should keep existing keys")
		assert.Equal(t, "existing", kb.Keys[0].Code)
	})

	t.Run("no changes for complete keybind", func(t *testing.T) {
		t.Parallel()

		kb := &keys.KeyBind{
			Description: "existing",
			Keys:        []keys.Key{keys.New("existing")},
		}
		originalKb := *kb
		defaultKb := keys.NewBind("default", keys.New("d"))

		keys.SetDefaultBind(&kb, defaultKb)

		assert.Equal(t, originalKb.Description, kb.Description, "should not change existing description")
		assert.Equal(t, originalKb.Keys, kb.Keys, "should not change existing keys")
	})

	t.Run("set both keys and description", func(t *testing.T) {
		t.Parallel()

		kb := &keys.KeyBind{
			Description: "",
			Keys:        []keys.Key{},
		}
		defaultKb := keys.NewBind("default", keys.New("d"))

		keys.SetDefaultBind(&kb, defaultKb)

		assert.Equal(t, "default", kb.Description, "should set default description")
		assert.Len(t, kb.Keys, 1, "should set default keys")
		assert.Equal(t, "d", kb.Keys[0].Code)
	})
}

func TestKeyBindRenderer_AddColumn(t *testing.T) {
	t.Parallel()

	t.Run("add single column with keybinds", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer

		kb1 := keys.NewBind("quit", keys.New("q"))
		kb2 := keys.NewBind("help", keys.New("h"))

		kbr.AddColumn(kb1, kb2)

		// We can't directly access columns since it's unexported, but we can test through Render
		result := kbr.Render(40)
		assert.Contains(t, result, "q")
		assert.Contains(t, result, "quit")
		assert.Contains(t, result, "h")
		assert.Contains(t, result, "help")
	})

	t.Run("add multiple columns", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer

		col1 := []keys.KeyBind{
			keys.NewBind("quit", keys.New("q")),
			keys.NewBind("help", keys.New("h")),
		}
		col2 := []keys.KeyBind{
			keys.NewBind("up", keys.New("k")),
			keys.NewBind("down", keys.New("j")),
		}

		kbr.AddColumn(col1...)
		kbr.AddColumn(col2...)

		result := kbr.Render(80)
		assert.Contains(t, result, "q")
		assert.Contains(t, result, "quit")
		assert.Contains(t, result, "k")
		assert.Contains(t, result, "up")
	})

	t.Run("add empty column does nothing", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer

		kb := keys.NewBind("quit", keys.New("q"))

		kbr.AddColumn(kb)

		resultBefore := kbr.Render(40)

		kbr.AddColumn() // Add empty column

		resultAfter := kbr.Render(40)
		assert.Equal(t, resultBefore, resultAfter, "adding empty column should not change output")
	})

	t.Run("initialize columns slice when nil", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer

		kb := keys.NewBind("quit", keys.New("q"))

		// First AddColumn call should initialize the columns slice
		kbr.AddColumn(kb)

		result := kbr.Render(40)
		assert.Contains(t, result, "q")
		assert.Contains(t, result, "quit")
	})
}

func TestKeyBindRenderer_Render(t *testing.T) {
	t.Parallel()

	t.Run("render single column", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer
		kbr.AddColumn(
			keys.NewBind("quit", keys.New("q")),
			keys.NewBind("help", keys.New("h")),
		)

		result := kbr.Render(40)

		// Check that both keybinds are present
		assert.Contains(t, result, "q")
		assert.Contains(t, result, "quit")
		assert.Contains(t, result, "h")
		assert.Contains(t, result, "help")

		// Check formatting
		lines := strings.Split(strings.TrimSpace(result), "\n")
		assert.Len(t, lines, 2, "should have 2 lines for 2 keybinds")
	})

	t.Run("render multiple columns", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer
		kbr.AddColumn(
			keys.NewBind("quit", keys.New("q")),
			keys.NewBind("help", keys.New("h")),
		)
		kbr.AddColumn(
			keys.NewBind("up", keys.New("k")),
			keys.NewBind("down", keys.New("j")),
		)

		result := kbr.Render(80)

		// Check that all keybinds are present
		assert.Contains(t, result, "q")
		assert.Contains(t, result, "quit")
		assert.Contains(t, result, "h")
		assert.Contains(t, result, "help")
		assert.Contains(t, result, "k")
		assert.Contains(t, result, "up")
		assert.Contains(t, result, "j")
		assert.Contains(t, result, "down")

		// The renderer puts each column on its own line, not side by side
		lines := strings.Split(strings.TrimSpace(result), "\n")
		assert.GreaterOrEqual(t, len(lines), 2, "should have multiple lines for multiple columns")
	})

	t.Run("render with keybinds containing aliases", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer
		kbr.AddColumn(
			keys.NewBind("up", keys.New("k"), keys.New("up", keys.WithAlias("↑"))),
			keys.NewBind("down", keys.New("j"), keys.New("down", keys.WithAlias("↓"))),
		)

		result := kbr.Render(40)

		// Should show aliases instead of codes
		assert.Contains(t, result, "↑")
		assert.Contains(t, result, "↓")
		assert.Contains(t, result, "k")
		assert.Contains(t, result, "j")
	})

	t.Run("render with hidden keys", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer
		kbr.AddColumn(
			keys.NewBind("quit", keys.New("q"), keys.New("esc", keys.Hidden())),
			keys.NewBind("help", keys.New("h")),
		)

		result := kbr.Render(40)

		// Hidden key should not appear
		assert.Contains(t, result, "q")
		assert.NotContains(t, result, "esc")
		assert.Contains(t, result, "h")
	})

	t.Run("render with varying column heights", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer
		// First column has 3 keybinds
		kbr.AddColumn(
			keys.NewBind("quit", keys.New("q")),
			keys.NewBind("help", keys.New("h")),
			keys.NewBind("refresh", keys.New("r")),
		)
		// Second column has 1 keybind
		kbr.AddColumn(
			keys.NewBind("up", keys.New("k")),
		)
		// Third column has 2 keybinds
		kbr.AddColumn(
			keys.NewBind("go back", keys.New("esc")),
			keys.NewBind("down", keys.New("down", keys.WithAlias("↓")), keys.New("j")),
		)

		result := kbr.Render(60)
		assert.Equal(t, ""+
			" q  quit             k  up               esc  go back       \n"+
			" h  help                                 ↓/j  down          \n"+
			" r  refresh                                                 ", result)
	})

	t.Run("render with narrow width", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer
		kbr.AddColumn(
			keys.NewBind("quit application", keys.New("q")),
		)

		result := kbr.Render(20)

		// Should still contain the key and description (possibly truncated)
		assert.Contains(t, result, "q")
		// Description might be truncated but should still contain some part
		assert.True(t, strings.Contains(result, "quit") || strings.Contains(result, "appli"))
	})

	t.Run("render empty renderer", func(t *testing.T) {
		t.Parallel()

		var kbr keys.KeyBindRenderer

		assert.Empty(t, kbr.Render(40), "should render empty string for empty renderer")
	})
}
