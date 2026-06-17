// Package filepicker provides a file picker component for Bubble Tea
// applications.
package filepicker

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"charm.land/lipgloss/v2"
	"github.com/dustin/go-humanize"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kat/pkg/keys"
)

// FilteredFS is a filesystem interface that supports filtered directory
// reading.
type FilteredFS interface {
	Close() error
	Name() string
	Open(name string) (*os.File, error)
	OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)
	Stat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]os.DirEntry, error)
}

var lastID atomic.Int64

func nextID() int {
	return int(lastID.Add(1))
}

// New returns a new filepicker model with default styling and key bindings.
func New(fsys FilteredFS) Model {
	return Model{
		id:               nextID(),
		fsys:             fsys,
		CurrentDirectory: ".",
		Cursor:           ">",
		AllowedTypes:     []string{},
		selected:         0,
		ShowPermissions:  true,
		ShowSize:         true,
		ShowHidden:       false,
		DirAllowed:       false,
		FileAllowed:      true,
		AutoHeight:       true,
		height:           0,
		maxIdx:           0,
		minIdx:           0,
		KeyMap:           DefaultKeyMap(),
		Styles:           DefaultStyles(),
	}
}

type errorMsg struct {
	err error
}

type readDirMsg struct {
	entries []os.DirEntry
	id      int
}

const (
	marginBottom = 5

	// FileSizeWidth is the column width for file sizes.
	FileSizeWidth = 7

	// PaddingLeft is the left padding for empty directory messages.
	PaddingLeft = 2
)

// KeyMap defines key bindings for each user action.
type KeyMap struct {
	GoToTop  keys.KeyBind
	GoToLast keys.KeyBind
	Down     keys.KeyBind
	Up       keys.KeyBind
	PageUp   keys.KeyBind
	PageDown keys.KeyBind
	Back     keys.KeyBind
	Open     keys.KeyBind
	Select   keys.KeyBind
}

// DefaultKeyMap defines the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		GoToTop:  keys.NewBind("first", keys.New("g")),
		GoToLast: keys.NewBind("last", keys.New("G")),
		Down:     keys.NewBind("down", keys.New("j"), keys.New("down"), keys.New("ctrl+n")),
		Up:       keys.NewBind("up", keys.New("k"), keys.New("up"), keys.New("ctrl+p")),
		PageUp:   keys.NewBind("page up", keys.New("K"), keys.New("pgup")),
		PageDown: keys.NewBind("page down", keys.New("J"), keys.New("pgdown")),
		Back:     keys.NewBind("back", keys.New("h"), keys.New("backspace"), keys.New("left"), keys.New("esc")),
		Open:     keys.NewBind("open", keys.New("l"), keys.New("right"), keys.New("enter")),
		Select:   keys.NewBind("select", keys.New("enter")),
	}
}

// Styles defines the possible customizations for styles in the file picker.
type Styles struct {
	DisabledCursor   lipgloss.Style
	Cursor           lipgloss.Style
	Symlink          lipgloss.Style
	Directory        lipgloss.Style
	File             lipgloss.Style
	DisabledFile     lipgloss.Style
	Permission       lipgloss.Style
	Selected         lipgloss.Style
	DisabledSelected lipgloss.Style
	FileSize         lipgloss.Style
	EmptyDirectory   lipgloss.Style
}

// DefaultStyles defines the default styling for the file picker.
func DefaultStyles() Styles {
	return Styles{
		DisabledCursor:   lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
		Cursor:           lipgloss.NewStyle().Foreground(lipgloss.Color("212")),
		Symlink:          lipgloss.NewStyle().Foreground(lipgloss.Color("36")),
		Directory:        lipgloss.NewStyle().Foreground(lipgloss.Color("99")),
		File:             lipgloss.NewStyle(),
		DisabledFile:     lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		DisabledSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
		Permission:       lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Selected:         lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
		FileSize: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Width(FileSizeWidth).
			Align(lipgloss.Right),
		EmptyDirectory: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			PaddingLeft(PaddingLeft).
			SetString("Bummer. No Files Found."),
	}
}

// Model represents a file picker.
type Model struct {
	Styles        Styles
	minStack      viewStack
	maxStack      viewStack
	selectedStack viewStack
	fsys          FilteredFS
	FileSelected  string

	// Path is the path which the user has selected with the file picker.
	Path string

	// CurrentDirectory is the directory that the user is currently in.
	CurrentDirectory string

	Cursor string
	files  []os.DirEntry

	// AllowedTypes specifies which file types the user may select.
	// If empty the user may select any file.
	AllowedTypes []string

	KeyMap          KeyMap
	id              int
	selected        int
	minIdx          int
	height          int
	maxIdx          int
	ShowSize        bool
	DirAllowed      bool
	ShowPermissions bool
	ShowHidden      bool
	AutoHeight      bool
	FileAllowed     bool
}

// viewStack is a simple integer stack used for tracking view state when
// navigating into directories.
type viewStack []int

func (s *viewStack) push(v int) {
	*s = append(*s, v)
}

func (s *viewStack) pop() int {
	old := *s
	v := old[len(old)-1]
	*s = old[:len(old)-1]

	return v
}

func (s viewStack) len() int {
	return len(s)
}

func (m *Model) pushView(selected, minimum, maximum int) {
	m.selectedStack.push(selected)
	m.minStack.push(minimum)
	m.maxStack.push(maximum)
}

func (m *Model) popView() (int, int, int) {
	return m.selectedStack.pop(), m.minStack.pop(), m.maxStack.pop()
}

// symlinkInfo holds the resolved symlink path and whether the target is a
// directory.
type symlinkInfo struct {
	path  string
	isDir bool
}

// resolveSymlink evaluates the symlink at the given path and returns the
// resolved target path and whether it is a directory. Returns nil if the entry
// is not a symlink or if resolution fails.
func resolveSymlink(dir, name string, info fs.FileInfo) *symlinkInfo {
	if info.Mode()&os.ModeSymlink == 0 {
		return nil
	}

	resolved, err := filepath.EvalSymlinks(filepath.Join(dir, name))
	if err != nil {
		slog.Error("resolve symlink: eval",
			slog.String("file", name),
			slog.Any("error", err),
		)

		return nil
	}

	target, err := os.Stat(resolved)
	if err != nil {
		slog.Error("resolve symlink: stat",
			slog.String("file", name),
			slog.Any("error", err),
		)

		return nil
	}

	return &symlinkInfo{path: resolved, isDir: target.IsDir()}
}

func (m Model) readDir(path string, _ bool) tea.Cmd {
	return func() tea.Msg {
		dirEntries, err := m.fsys.ReadDir(path)
		if err != nil {
			return errorMsg{err}
		}

		sort.Slice(dirEntries, func(i, j int) bool {
			if dirEntries[i].IsDir() == dirEntries[j].IsDir() {
				return dirEntries[i].Name() < dirEntries[j].Name()
			}

			return dirEntries[i].IsDir()
		})

		return readDirMsg{id: m.id, entries: dirEntries}
	}
}

// Init initializes the file picker model.
func (m Model) Init() tea.Cmd {
	return m.readDir(m.CurrentDirectory, m.ShowHidden)
}

// SetHeight sets the height of the filepicker.
func (m *Model) SetHeight(h int) {
	m.height = h
	if m.maxIdx > m.height-1 {
		m.maxIdx = m.minIdx + m.height - 1
	}
}

// Height returns the height of the filepicker.
func (m Model) Height() int {
	return m.height
}

// Update handles user interactions within the file picker model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case errorMsg:
		slog.Error("read directory", slog.Any("error", msg.err))

	case readDirMsg:
		if msg.id != m.id {
			break
		}

		m.files = msg.entries
		m.maxIdx = max(m.maxIdx, m.height-1)

	case tea.WindowSizeMsg:
		if m.AutoHeight {
			m.SetHeight(msg.Height - marginBottom)
		}

		m.maxIdx = m.height - 1

	case tea.KeyPressMsg:
		k := msg.String()

		switch {
		case m.KeyMap.GoToTop.Match(k):
			m.selected = 0
			m.minIdx = 0
			m.maxIdx = m.height - 1

		case m.KeyMap.GoToLast.Match(k):
			m.selected = len(m.files) - 1
			m.minIdx = len(m.files) - m.height
			m.maxIdx = len(m.files) - 1

		case m.KeyMap.Down.Match(k):
			m.selected++
			if m.selected >= len(m.files) {
				m.selected = len(m.files) - 1
			}

			if m.selected > m.maxIdx {
				m.minIdx++
				m.maxIdx++
			}

		case m.KeyMap.Up.Match(k):
			m.selected--
			if m.selected < 0 {
				m.selected = 0
			}

			if m.selected < m.minIdx {
				m.minIdx--
				m.maxIdx--
			}

		case m.KeyMap.PageDown.Match(k):
			m.selected += m.height
			if m.selected >= len(m.files) {
				m.selected = len(m.files) - 1
			}

			m.minIdx += m.height
			m.maxIdx += m.height

			if m.maxIdx >= len(m.files) {
				m.maxIdx = len(m.files) - 1
				m.minIdx = m.maxIdx - m.height
			}

		case m.KeyMap.PageUp.Match(k):
			m.selected -= m.height
			if m.selected < 0 {
				m.selected = 0
			}

			m.minIdx -= m.height
			m.maxIdx -= m.height

			if m.minIdx < 0 {
				m.minIdx = 0
				m.maxIdx = m.minIdx + m.height
			}

		case m.KeyMap.Back.Match(k):
			m.CurrentDirectory = filepath.Dir(m.CurrentDirectory)
			if m.selectedStack.len() > 0 {
				m.selected, m.minIdx, m.maxIdx = m.popView()
			} else {
				m.selected = 0
				m.minIdx = 0
				m.maxIdx = m.height - 1
			}

			return m, m.readDir(m.CurrentDirectory, m.ShowHidden)

		case m.KeyMap.Open.Match(k):
			if len(m.files) == 0 {
				break
			}

			f := m.files[m.selected]
			info, err := f.Info()
			if err != nil {
				break
			}

			isDir := f.IsDir()

			if sl := resolveSymlink(m.CurrentDirectory, f.Name(), info); sl != nil {
				if sl.isDir {
					isDir = true
				}
			}

			if (!isDir && m.FileAllowed) || (isDir && m.DirAllowed) {
				if m.KeyMap.Select.Match(k) {
					// Select the current path as the selection.
					m.Path = filepath.Join(m.CurrentDirectory, f.Name())
				}
			}

			if !isDir {
				break
			}

			m.CurrentDirectory = filepath.Join(m.CurrentDirectory, f.Name())
			m.pushView(m.selected, m.minIdx, m.maxIdx)

			m.selected = 0
			m.minIdx = 0
			m.maxIdx = m.height - 1

			return m, m.readDir(m.CurrentDirectory, m.ShowHidden)
		}
	}

	return m, nil
}

// View returns the view of the file picker.
func (m Model) View() tea.View {
	if len(m.files) == 0 {
		return tea.NewView(m.Styles.EmptyDirectory.Height(m.height).MaxHeight(m.height).String())
	}

	var s strings.Builder

	for i, f := range m.files {
		if i < m.minIdx || i > m.maxIdx {
			continue
		}

		var symlinkPath string

		name := f.Name()

		info, err := f.Info()
		if err != nil {
			slog.Error("get file info",
				slog.String("file", name),
				slog.Any("error", err),
			)

			continue
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0

		//nolint:gosec // Checked with max.
		size := strings.Replace(humanize.Bytes(uint64(max(0, info.Size()))), " ", "", 1)

		if sl := resolveSymlink(m.CurrentDirectory, name, info); sl != nil {
			symlinkPath = sl.path
		}

		disabled := !m.canSelect(name) && !f.IsDir()

		if m.selected == i {
			selected := ""
			if m.ShowPermissions {
				selected += " " + info.Mode().String()
			}

			if m.ShowSize {
				selected += fmt.Sprintf("%"+strconv.Itoa(m.Styles.FileSize.GetWidth())+"s", size)
			}

			selected += " " + name
			if isSymlink {
				selected += " → " + symlinkPath
			}

			if disabled {
				s.WriteString(m.Styles.DisabledSelected.Render(m.Cursor) + m.Styles.DisabledSelected.Render(selected))
			} else {
				s.WriteString(m.Styles.Cursor.Render(m.Cursor) + m.Styles.Selected.Render(selected))
			}

			s.WriteRune('\n')

			continue
		}

		style := m.Styles.File
		switch {
		case f.IsDir():
			style = m.Styles.Directory
		case isSymlink:
			style = m.Styles.Symlink
		case disabled:
			style = m.Styles.DisabledFile
		}

		fileName := style.Render(name)
		s.WriteString(m.Styles.Cursor.Render(" "))

		if isSymlink {
			fileName += " → " + symlinkPath
		}

		if m.ShowPermissions {
			s.WriteString(" " + m.Styles.Permission.Render(info.Mode().String()))
		}

		if m.ShowSize {
			s.WriteString(m.Styles.FileSize.Render(size))
		}

		s.WriteString(" " + fileName)
		s.WriteRune('\n')
	}

	for i := lipgloss.Height(s.String()); i <= m.height; i++ {
		s.WriteRune('\n')
	}

	return tea.NewView(s.String())
}

// HighlightedPath returns the path of the currently highlighted file or directory.
func (m Model) HighlightedPath() string {
	if len(m.files) == 0 || m.selected < 0 || m.selected >= len(m.files) {
		return ""
	}

	return filepath.Join(m.CurrentDirectory, m.files[m.selected].Name())
}

// DidSelectFile returns whether a user has selected a file (on this msg).
func (m Model) DidSelectFile(msg tea.Msg) (bool, string) {
	didSelect, path := m.didSelectAnyFile(msg)
	if didSelect && m.canSelect(path) {
		return true, path
	}

	return false, ""
}

// DidSelectDisabledFile returns whether a user tried to select a disabled file
// (on this msg). This is necessary only if you would like to warn the user that
// they tried to select a disabled file.
func (m Model) DidSelectDisabledFile(msg tea.Msg) (bool, string) {
	didSelect, path := m.didSelectAnyFile(msg)
	if didSelect && !m.canSelect(path) {
		return true, path
	}

	return false, ""
}

func (m Model) didSelectAnyFile(msg tea.Msg) (bool, string) {
	if len(m.files) == 0 {
		return false, ""
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// If the msg does not match the Select keymap then this could not have been a selection.
		if !m.KeyMap.Select.Match(msg.String()) {
			return false, ""
		}

		// The key press was a selection, let's confirm whether the current file could
		// be selected or used for navigating deeper into the stack.
		f := m.files[m.selected]
		info, err := f.Info()
		if err != nil {
			return false, ""
		}

		isDir := f.IsDir()

		if sl := resolveSymlink(m.CurrentDirectory, f.Name(), info); sl != nil {
			if sl.isDir {
				isDir = true
			}
		}

		if ((!isDir && m.FileAllowed) || (isDir && m.DirAllowed)) && m.Path != "" {
			return true, m.Path
		}

		// If the msg was not a KeyMsg, then the file could not have been selected this iteration.
		// Only a KeyMsg can select a file.

	default:
		return false, ""
	}

	return false, ""
}

func (m Model) canSelect(file string) bool {
	if len(m.AllowedTypes) == 0 {
		return true
	}

	for _, ext := range m.AllowedTypes {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}

	return false
}
