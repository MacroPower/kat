package filepicker_test

import (
	"bytes"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	iofs "io/fs"

	"github.com/macropower/kat/pkg/ui/filepicker"
	"github.com/macropower/kat/pkg/uitest"
)

// mockFS implements FilteredFS for testing.
type mockFS struct {
	entries map[string][]mockEntry
}

type mockEntry struct {
	name  string
	size  int64
	mode  iofs.FileMode
	isDir bool
}

func newMockFS(entries map[string][]mockEntry) *mockFS {
	if entries == nil {
		entries = make(map[string][]mockEntry)
	}

	return &mockFS{entries: entries}
}

func (m *mockFS) Close() error {
	return nil
}

func (m *mockFS) Name() string {
	return "mock"
}

func (m *mockFS) Open(_ string) (*os.File, error) {
	return nil, os.ErrNotExist
}

func (m *mockFS) OpenFile(_ string, _ int, _ iofs.FileMode) (*os.File, error) {
	return nil, os.ErrNotExist
}

func (m *mockFS) Stat(_ string) (iofs.FileInfo, error) {
	return nil, os.ErrNotExist
}

func (m *mockFS) ReadDir(name string) ([]os.DirEntry, error) {
	entries, ok := m.entries[name]
	if !ok {
		return []os.DirEntry{}, nil
	}

	result := make([]os.DirEntry, len(entries))
	for i, e := range entries {
		result[i] = &mockDirEntry{entry: e}
	}

	return result, nil
}

// mockDirEntry implements os.DirEntry.
type mockDirEntry struct {
	entry mockEntry
}

func (d *mockDirEntry) Name() string {
	return d.entry.name
}

func (d *mockDirEntry) IsDir() bool {
	return d.entry.isDir
}

func (d *mockDirEntry) Type() iofs.FileMode {
	if d.entry.isDir {
		return iofs.ModeDir
	}

	return d.entry.mode
}

func (d *mockDirEntry) Info() (iofs.FileInfo, error) {
	return &mockFileInfo{entry: d.entry}, nil
}

// mockFileInfo implements iofs.FileInfo.
type mockFileInfo struct {
	entry mockEntry
}

func (f *mockFileInfo) Name() string {
	return f.entry.name
}

func (f *mockFileInfo) Size() int64 {
	return f.entry.size
}

func (f *mockFileInfo) Mode() iofs.FileMode {
	if f.entry.isDir {
		return iofs.ModeDir | 0o755
	}

	return f.entry.mode
}

func (f *mockFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (f *mockFileInfo) IsDir() bool {
	return f.entry.isDir
}

func (f *mockFileInfo) Sys() any {
	return nil
}

func TestFilepicker_View_Empty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		golden string
		size   uitest.Size
	}{
		{
			name:   "empty directory compact",
			size:   uitest.Compact,
			golden: "empty_compact",
		},
		{
			name:   "empty directory standard",
			size:   uitest.Standard,
			golden: "empty_standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fs := newMockFS(nil)
			m := filepicker.New(fs)

			tm := uitest.NewTestModel(t, m, tt.size)

			// Send window size to trigger render
			tm.Send(tea.WindowSizeMsg{
				Width:  tt.size.Width,
				Height: tt.size.Height,
			})

			// Quit to get final output
			tm.Send(tea.QuitMsg{})

			output := uitest.GetFinalOutput(t, tm, time.Second)
			uitest.AssertGolden(t, tt.golden, output)
		})
	}
}

func TestFilepicker_Navigation(t *testing.T) {
	t.Parallel()

	fs := newMockFS(map[string][]mockEntry{
		".": {
			{name: "dir1", isDir: true, mode: 0o755},
			{name: "dir2", isDir: true, mode: 0o755},
			{name: "file1.txt", size: 1024, mode: 0o644},
			{name: "file2.go", size: 2048, mode: 0o644},
		},
	})

	m := filepicker.New(fs)
	tm := uitest.NewTestModel(t, m, uitest.Compact)

	// Send window size
	tm.Send(tea.WindowSizeMsg{
		Width:  uitest.CompactWidth,
		Height: uitest.CompactHeight,
	})

	// Capture initial state with files loaded
	output := uitest.WaitForCapture(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("dir1"))
	})
	uitest.AssertGolden(t, "navigation_01_initial", output)

	// Navigate down once (j key)
	tm.Type("j")

	output = uitest.WaitForCapture(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("dir2"))
	})
	uitest.AssertGolden(t, "navigation_02_down_once", output)

	// Navigate down again
	tm.Type("j")

	output = uitest.WaitForCapture(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("file1"))
	})
	uitest.AssertGolden(t, "navigation_03_down_twice", output)

	// Go to last item (G key)
	tm.Type("G")

	output = uitest.WaitForCapture(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("file2"))
	})
	uitest.AssertGolden(t, "navigation_04_go_to_last", output)

	// Go to top (g key)
	tm.Type("g")

	output = uitest.WaitForCapture(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("dir1"))
	})
	uitest.AssertGolden(t, "navigation_05_go_to_top", output)

	tm.Send(tea.QuitMsg{})
}
