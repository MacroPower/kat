package filepicker_test

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kat/pkg/ui/filepicker"
)

var errNotImplemented = errors.New("not implemented")

// stubFileInfo implements [fs.FileInfo] for testing.
type stubFileInfo struct {
	name string
	mode fs.FileMode
	dir  bool
}

func (s stubFileInfo) Name() string       { return s.name }
func (s stubFileInfo) Size() int64        { return 0 }
func (s stubFileInfo) Mode() fs.FileMode  { return s.mode }
func (s stubFileInfo) ModTime() time.Time { return time.Time{} }
func (s stubFileInfo) IsDir() bool        { return s.dir }
func (s stubFileInfo) Sys() any           { return nil }

// stubDirEntry implements [os.DirEntry] for testing.
type stubDirEntry struct {
	name string
	dir  bool
	mode fs.FileMode
}

func (s stubDirEntry) Name() string      { return s.name }
func (s stubDirEntry) IsDir() bool       { return s.dir }
func (s stubDirEntry) Type() fs.FileMode { return s.mode.Type() }
func (s stubDirEntry) Info() (fs.FileInfo, error) {
	return stubFileInfo{name: s.name, mode: s.mode, dir: s.dir}, nil
}

// stubFS implements [filepicker.FilteredFS] for testing.
type stubFS struct {
	entries []os.DirEntry
}

func (s stubFS) Close() error                                        { return nil }
func (s stubFS) Name() string                                        { return "stub" }
func (s stubFS) Open(string) (*os.File, error)                       { return nil, errNotImplemented }
func (s stubFS) OpenFile(string, int, fs.FileMode) (*os.File, error) { return nil, errNotImplemented }
func (s stubFS) Stat(string) (fs.FileInfo, error)                    { return nil, errNotImplemented }
func (s stubFS) ReadDir(string) ([]os.DirEntry, error)               { return s.entries, nil }

func enterKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter}
}

func TestDidSelectFile_PrecedenceBug(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		input       filepicker.Model
		wantSelect  bool
		wantDisable bool
	}{
		"file allowed but path empty returns false": {
			input: func() filepicker.Model {
				m := filepicker.New(stubFS{entries: []os.DirEntry{
					stubDirEntry{name: "test.yaml", dir: false, mode: 0o644},
				}})
				m.FileAllowed = true
				m.DirAllowed = false
				m.Path = ""

				return m
			}(),
			wantSelect:  false,
			wantDisable: false,
		},
		"file allowed and path set returns true": {
			input: func() filepicker.Model {
				m := filepicker.New(stubFS{entries: []os.DirEntry{
					stubDirEntry{name: "test.yaml", dir: false, mode: 0o644},
				}})
				m.FileAllowed = true
				m.DirAllowed = false
				m.Path = "/some/path/test.yaml"

				return m
			}(),
			wantSelect:  true,
			wantDisable: false,
		},
		"dir allowed but path empty returns false": {
			input: func() filepicker.Model {
				m := filepicker.New(stubFS{entries: []os.DirEntry{
					stubDirEntry{name: "subdir", dir: true, mode: fs.ModeDir | 0o755},
				}})
				m.FileAllowed = false
				m.DirAllowed = true
				m.Path = ""

				return m
			}(),
			wantSelect:  false,
			wantDisable: false,
		},
		"neither allowed returns false": {
			input: func() filepicker.Model {
				m := filepicker.New(stubFS{entries: []os.DirEntry{
					stubDirEntry{name: "test.yaml", dir: false, mode: 0o644},
				}})
				m.FileAllowed = false
				m.DirAllowed = false
				m.Path = "/some/path"

				return m
			}(),
			wantSelect:  false,
			wantDisable: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := tc.input

			// Execute Init cmd to populate files.
			cmd := m.Init()
			if cmd != nil {
				msg := cmd()
				m, _ = m.Update(msg)
			}

			msg := enterKeyMsg()
			gotSelect, _ := m.DidSelectFile(msg)
			gotDisable, _ := m.DidSelectDisabledFile(msg)

			assert.Equal(t, tc.wantSelect, gotSelect, "DidSelectFile")
			assert.Equal(t, tc.wantDisable, gotDisable, "DidSelectDisabledFile")
		})
	}
}
