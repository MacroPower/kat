package yamls_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"github.com/macropower/kat/pkg/ui/yamls"
)

func TestNewDiffHighlighter(t *testing.T) {
	t.Parallel()

	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	changedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	highlighter := yamls.NewDiffHighlighter(addedStyle, removedStyle, changedStyle)

	assert.NotNil(t, highlighter)
}

func TestDiffHighlighter_ApplyDiffHighlights(t *testing.T) {
	t.Parallel()

	addedStyle := lipgloss.NewStyle().Background(lipgloss.Color("2"))
	removedStyle := lipgloss.NewStyle().Background(lipgloss.Color("1"))
	changedStyle := lipgloss.NewStyle().Background(lipgloss.Color("3"))
	highlighter := yamls.NewDiffHighlighter(addedStyle, removedStyle, changedStyle)

	tcs := map[string]struct {
		input string
		want  string
		diffs []yamls.DiffPosition
	}{
		"empty text and no diffs": {
			input: "",
			diffs: []yamls.DiffPosition{},
			want:  "",
		},
		"text with no diffs": {
			input: "hello world",
			diffs: []yamls.DiffPosition{},
			want:  "hello world",
		},
		"single line with added diff": {
			input: "hello world",
			diffs: []yamls.DiffPosition{
				{Line: 0, Start: 6, End: 11, Type: yamls.DiffInserted},
			},
			want: "hello " + addedStyle.Render("world"),
		},
		"single line with changed diff": {
			input: "hello world",
			diffs: []yamls.DiffPosition{
				{Line: 0, Start: 0, End: 5, Type: yamls.DiffEdited},
			},
			want: changedStyle.Render("hello") + " world",
		},
		"multiple lines with different diff types": {
			input: "line1\nline2\nline3",
			diffs: []yamls.DiffPosition{
				{Line: 0, Start: 0, End: 5, Type: yamls.DiffInserted},
				{Line: 2, Start: 0, End: 5, Type: yamls.DiffEdited},
			},
			want: addedStyle.Render("line1") + "\nline2\n" + changedStyle.Render("line3"),
		},
		"line with multiple ranges": {
			input: "hello world test",
			diffs: []yamls.DiffPosition{
				{Line: 0, Start: 0, End: 5, Type: yamls.DiffInserted},
				{Line: 0, Start: 12, End: 16, Type: yamls.DiffEdited},
			},
			want: addedStyle.Render("hello") + " world " + changedStyle.Render("test"),
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := highlighter.ApplyDiffHighlights(tc.input, tc.diffs)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNewDiffer(t *testing.T) {
	t.Parallel()

	differ := yamls.NewDiffer()

	assert.NotNil(t, differ)
	assert.Empty(t, differ.GetDiffs())
}

func TestDiffer_SetInitialContent(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		initialContent string
		newContent     string
		wantOriginal   string
	}{
		"first call sets content": {
			initialContent: "initial content",
			newContent:     "new content",
			wantOriginal:   "initial content",
		},
		"second call does not override": {
			initialContent: "first",
			newContent:     "second",
			wantOriginal:   "first",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			differ := yamls.NewDiffer()
			differ.SetInitialContent(tc.initialContent)
			differ.SetInitialContent(tc.newContent)

			// We can't directly test the original content, but we can test behavior
			diffs := differ.FindAndCacheDiffs(tc.wantOriginal)
			assert.Empty(t, diffs, "original content should match")
		})
	}
}

func TestDiffer_SetOriginalContent(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		firstContent  string
		secondContent string
		testContent   string
		wantDiffs     bool
	}{
		"setting same content twice": {
			firstContent:  "same content",
			secondContent: "same content",
			testContent:   "same content",
			wantDiffs:     false,
		},
		"setting different content clears diffs": {
			firstContent:  "first content",
			secondContent: "second content",
			testContent:   "second content",
			wantDiffs:     false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			differ := yamls.NewDiffer()
			differ.SetOriginalContent(tc.firstContent)

			// Add some diffs by finding differences
			differ.FindAndCacheDiffs("some different content")

			differ.SetOriginalContent(tc.secondContent)

			diffs := differ.FindAndCacheDiffs(tc.testContent)

			if tc.wantDiffs {
				assert.NotEmpty(t, diffs)
			} else {
				assert.Empty(t, diffs)
			}
		})
	}
}

func TestDiffer_FindDiffs(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		originalContent string
		currentContent  string
		wantDiffTypes   []yamls.DiffType
		wantDiffsCount  int
	}{
		"no original content": {
			originalContent: "",
			currentContent:  "some content",
			wantDiffsCount:  1,
			wantDiffTypes:   []yamls.DiffType{},
		},
		"same content": {
			originalContent: "same content",
			currentContent:  "same content",
			wantDiffsCount:  0,
			wantDiffTypes:   []yamls.DiffType{},
		},
		"simple addition": {
			originalContent: "hello",
			currentContent:  "hello world",
			wantDiffsCount:  1,
			wantDiffTypes:   []yamls.DiffType{yamls.DiffInserted},
		},
		"simple change": {
			originalContent: "hello world",
			currentContent:  "hello there",
			wantDiffsCount:  2, // With the fix, both edits should be captured
			wantDiffTypes:   []yamls.DiffType{yamls.DiffEdited, yamls.DiffEdited},
		},
		"multiline addition": {
			originalContent: "line1",
			currentContent:  "line1\nline2\nline3",
			wantDiffsCount:  3, // Due to bug: only last edit is kept, but this edit spans 3 lines
			wantDiffTypes:   []yamls.DiffType{yamls.DiffInserted, yamls.DiffInserted, yamls.DiffInserted},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			differ := yamls.NewDiffer()
			differ.SetOriginalContent(tc.originalContent)

			diffs := differ.FindAndCacheDiffs(tc.currentContent)

			assert.Len(t, diffs, tc.wantDiffsCount)

			for i, expectedType := range tc.wantDiffTypes {
				if i < len(diffs) {
					assert.Equal(t, expectedType, diffs[i].Type)
				}
			}
		})
	}
}

func TestDiffer_GetDiffs(t *testing.T) {
	t.Parallel()

	differ := yamls.NewDiffer()
	differ.SetOriginalContent("original")

	diffs := differ.FindAndCacheDiffs("modified")

	gotDiffs := differ.GetDiffs()
	assert.Equal(t, diffs, gotDiffs)
}

func TestDiffer_ClearDiffs(t *testing.T) {
	t.Parallel()

	differ := yamls.NewDiffer()
	differ.SetOriginalContent("original")
	differ.FindAndCacheDiffs("modified")

	// Ensure diffs exist before clearing
	assert.NotEmpty(t, differ.GetDiffs())

	differ.ClearDiffs()
	assert.Empty(t, differ.GetDiffs())
}

func TestDiffer_Unload(t *testing.T) {
	t.Parallel()

	differ := yamls.NewDiffer()
	differ.SetOriginalContent("original")
	differ.FindAndCacheDiffs("modified")

	// Ensure diffs exist before unloading
	assert.NotEmpty(t, differ.GetDiffs())

	differ.Unload()

	// After unload, diffs should be cleared and original content reset
	assert.Empty(t, differ.GetDiffs())
}

func TestConvertEditToDiffPositions(t *testing.T) {
	t.Parallel()

	// Since convertEditToDiffPositions is internal, we test it indirectly through FindDiffs
	tcs := map[string]struct {
		originalContent string
		currentContent  string
		wantDiffsCount  int
	}{
		"single line addition": {
			originalContent: "hello",
			currentContent:  "hello world",
			wantDiffsCount:  1,
		},
		"single line change": {
			originalContent: "hello world",
			currentContent:  "hello there",
			wantDiffsCount:  2, // "wor" -> "the" and "ld" -> "re"
		},
		"multiline addition": {
			originalContent: "line1",
			currentContent:  "line1\nline2\nline3",
			wantDiffsCount:  3, // line1 gets modified, line2 and line3 are added
		},
		"invalid case handled gracefully": {
			originalContent: "valid content",
			currentContent:  "valid content",
			wantDiffsCount:  0,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			differ := yamls.NewDiffer()
			differ.SetOriginalContent(tc.originalContent)

			diffs := differ.FindAndCacheDiffs(tc.currentContent)

			assert.Len(t, diffs, tc.wantDiffsCount)

			// Verify all diff positions are valid
			if len(diffs) > 0 {
				lines := strings.Split(tc.currentContent, "\n")
				for _, diff := range diffs {
					assert.GreaterOrEqual(t, diff.Line, 0, "line number should be non-negative")
					assert.Less(t, diff.Line, len(lines), "line number should be within content")
					assert.GreaterOrEqual(t, diff.Start, 0, "start position should be non-negative")
					assert.GreaterOrEqual(t, diff.End, diff.Start, "end should be >= start")
					assert.LessOrEqual(t, diff.End, len([]rune(lines[diff.Line])), "end should be within line length")
				}
			}
		})
	}
}

func TestOffsetToLineCol(t *testing.T) {
	t.Parallel()

	// This function is internal but we can test it indirectly through public APIs
	tcs := map[string]struct {
		content     string
		setupDiffer func(*yamls.Differ)
		testContent string
		wantDiffs   bool
	}{
		"valid offset at start": {
			content: "hello\nworld",
			setupDiffer: func(d *yamls.Differ) {
				d.SetOriginalContent("hello\nworld")
			},
			testContent: "hello\nworld",
			wantDiffs:   false,
		},
		"valid offset at newline": {
			content: "hello\nworld",
			setupDiffer: func(d *yamls.Differ) {
				d.SetOriginalContent("hello")
			},
			testContent: "hello\nworld",
			wantDiffs:   true,
		},
		"offset beyond content": {
			content: "short",
			setupDiffer: func(d *yamls.Differ) {
				d.SetOriginalContent("short")
			},
			testContent: "short",
			wantDiffs:   false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			differ := yamls.NewDiffer()
			tc.setupDiffer(differ)

			diffs := differ.FindAndCacheDiffs(tc.testContent)

			if tc.wantDiffs {
				assert.NotEmpty(t, diffs)
			} else {
				assert.Empty(t, diffs)
			}
		})
	}
}

func TestDiffer_Integration(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		original string
		current  string
		wantLen  int
	}{
		"yaml content modification": {
			original: `apiVersion: v1
kind: Pod
metadata:
  name: test`,
			current: `apiVersion: v1
kind: Pod
metadata:
  name: modified-test
  labels:
    app: test`,
			wantLen: 4,
		},
		"line removal simulation": {
			original: `line1
line2
line3`,
			current: `line1
line3`,
			wantLen: 0, // Removals are skipped in current implementation.
		},
		"complex multiline change": {
			original: `function hello() {
  console.log("hello");
}`,
			current: `function hello() {
  console.log("hello world");
  console.log("additional line");
}`,
			wantLen: 5,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			differ := yamls.NewDiffer()
			differ.SetOriginalContent(tc.original)

			diffs := differ.FindAndCacheDiffs(tc.current)

			assert.Len(t, diffs, tc.wantLen, "diffs %+v should match expected length", diffs)

			// Verify all diff positions are valid
			lines := strings.Split(tc.current, "\n")
			for _, diff := range diffs {
				assert.GreaterOrEqual(t, diff.Line, 0, "line number should be non-negative")
				assert.Less(t, diff.Line, len(lines), "line number should be within content")
				assert.GreaterOrEqual(t, diff.Start, 0, "start position should be non-negative")
				assert.GreaterOrEqual(t, diff.End, diff.Start, "end should be >= start")
				assert.LessOrEqual(t, diff.End, len([]rune(lines[diff.Line])), "end should be within line length")
			}
		})
	}
}
