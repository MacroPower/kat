package filepicker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/ui/theme"
)

type FilteredFS interface {
	Close() error
	Name() string
	Open(name string) (*os.File, error)
	OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)
	Stat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]os.DirEntry, error)
}

var lastID int64

func nextID() int {
	return int(atomic.AddInt64(&lastID, 1))
}

// New returns a new filepicker model with default styling and key bindings.
func New(fsys FilteredFS, t *theme.Theme) Model {
	return Model{
		id:               nextID(),
		fsys:             fsys,
		Theme:            t,
		rootPath:         "/",
		CurrentDirectory: ".",
		Cursor:           ">",
		AllowedTypes:     []string{},
		selected:         0,
		ShowPermissions:  true,
		ShowSize:         true,
		DirAllowed:       false,
		FileAllowed:      true,
		AutoHeight:       true,
		Height:           0,
		Width:            0,
		max:              0,
		min:              0,
		selectedStack:    newStack(),
		minStack:         newStack(),
		maxStack:         newStack(),
		Styles:           DefaultStyles(),
	}
}

type errorMsg struct {
	err error //nolint:unused // TODO: Use it.
}

type readDirMsg struct {
	entries []fs.DirEntry
	id      int
}

const (
	fileSizeWidth = 7
	paddingLeft   = 2
)

// KeyMap defines key bindings for each user action.
type KeyMap struct {
	GoToTop  key.Binding
	GoToLast key.Binding
	Down     key.Binding
	Up       key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Back     key.Binding
	Open     key.Binding
	Select   key.Binding
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
	return DefaultStylesWithRenderer(lipgloss.DefaultRenderer())
}

// DefaultStylesWithRenderer defines the default styling for the file picker,
// with a given Lip Gloss renderer.
func DefaultStylesWithRenderer(r *lipgloss.Renderer) Styles {
	return Styles{
		DisabledCursor:   r.NewStyle().Foreground(lipgloss.Color("247")),
		Cursor:           r.NewStyle().Foreground(lipgloss.Color("212")),
		Symlink:          r.NewStyle().Foreground(lipgloss.Color("36")),
		Directory:        r.NewStyle().Foreground(lipgloss.Color("99")),
		File:             r.NewStyle(),
		DisabledFile:     r.NewStyle().Foreground(lipgloss.Color("243")),
		DisabledSelected: r.NewStyle().Foreground(lipgloss.Color("247")),
		Permission:       r.NewStyle().Foreground(lipgloss.Color("244")),
		Selected:         r.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
		FileSize:         r.NewStyle().Foreground(lipgloss.Color("240")).Width(fileSizeWidth).Align(lipgloss.Right),
		EmptyDirectory: r.NewStyle().
			Foreground(lipgloss.Color("240")).
			PaddingLeft(paddingLeft).
			SetString("Bummer. No Files Found."),
	}
}

// Model represents a file picker.
type Model struct {
	Styles           Styles
	Theme            *theme.Theme
	selectedStack    stack
	minStack         stack
	maxStack         stack
	fsys             FilteredFS
	FileSelected     string
	Path             string
	Cursor           string
	CurrentDirectory string
	rootPath         string
	files            []fs.DirEntry
	AllowedTypes     []string
	Height           int
	Width            int
	max              int
	id               int
	min              int
	selected         int
	FileAllowed      bool
	DirAllowed       bool
	AutoHeight       bool
	ShowSize         bool
	ShowPermissions  bool
}

type stack struct {
	Push   func(int)
	Pop    func() int
	Length func() int
}

func newStack() stack {
	slice := make([]int, 0)

	return stack{
		Push: func(i int) {
			slice = append(slice, i)
		},
		Pop: func() int {
			res := slice[len(slice)-1]
			slice = slice[:len(slice)-1]
			return res
		},
		Length: func() int {
			return len(slice)
		},
	}
}

func (m *Model) pushView(selected, minimum, maximum int) {
	m.selectedStack.Push(selected)
	m.minStack.Push(minimum)
	m.maxStack.Push(maximum)
}

func (m *Model) popView() (int, int, int) {
	return m.selectedStack.Pop(), m.minStack.Pop(), m.maxStack.Pop()
}

func (m Model) readDir(path string) tea.Cmd {
	return func() tea.Msg {
		dirEntries, err := m.fsys.ReadDir(path)
		if err != nil {
			return errorMsg{err: err}
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
	return m.readDir(m.CurrentDirectory)
}

// SetSize sets the size of the filepicker.
func (m *Model) SetSize(w, h int) {
	m.Width = w
	m.Height = h
	if m.max > m.Height-1 {
		m.max = m.min + m.Height - 1
	} else {
		m.max = m.Height - 1
	}
}

// Update handles user interactions within the file picker model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case readDirMsg:
		if msg.id != m.id {
			break
		}

		m.files = msg.entries
		m.max = max(m.max, m.Height-1)
	}

	return m, nil
}

// GoToTop moves the cursor to the top of the file list.
func (m *Model) GoToTop() {
	m.selected = 0
	m.min = 0
	m.max = m.Height - 1
}

// GoToLast moves the cursor to the last item in the file list.
func (m *Model) GoToLast() {
	m.selected = len(m.files) - 1
	m.min = len(m.files) - m.Height
	m.max = len(m.files) - 1
}

// MoveDown moves the cursor down one item in the file list.
func (m *Model) MoveDown() {
	m.selected++
	if m.selected >= len(m.files) {
		m.selected = len(m.files) - 1
	}
	if m.selected > m.max {
		m.min++
		m.max++
	}
}

// MoveUp moves the cursor up one item in the file list.
func (m *Model) MoveUp() {
	m.selected--
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected < m.min {
		m.min--
		m.max--
	}
}

// PageDown moves the cursor down by one page in the file list.
func (m *Model) PageDown() {
	m.selected += m.Height
	if m.selected >= len(m.files) {
		m.selected = len(m.files) - 1
	}

	m.min += m.Height
	m.max += m.Height

	if m.max >= len(m.files) {
		m.max = len(m.files) - 1
		m.min = m.max - m.Height
	}
}

// PageUp moves the cursor up by one page in the file list.
func (m *Model) PageUp() {
	m.selected -= m.Height
	if m.selected < 0 {
		m.selected = 0
	}

	m.min -= m.Height
	m.max -= m.Height

	if m.min < 0 {
		m.min = 0
		m.max = m.min + m.Height
	}
}

// GoBack navigates to the parent directory.
func (m Model) GoBack() (Model, tea.Cmd) {
	m.CurrentDirectory = filepath.Dir(m.CurrentDirectory)
	if m.selectedStack.Length() > 0 {
		m.selected, m.min, m.max = m.popView()
	} else {
		m.selected = 0
		m.min = 0
		m.max = m.Height - 1
	}

	return m, m.readDir(m.CurrentDirectory)
}

// Open opens the currently selected file or directory.
func (m Model) Open() (Model, tea.Cmd) {
	if len(m.files) == 0 {
		return m, nil
	}

	f := m.files[m.selected]
	info, err := f.Info()
	if err != nil {
		return m, nil
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0
	isDir := f.IsDir()

	if isSymlink {
		// For symlinks, we need to resolve them using the actual filesystem.
		actualPath := filepath.Join(m.rootPath, m.CurrentDirectory, f.Name())
		symlinkPath, err := filepath.EvalSymlinks(actualPath)
		if err != nil {
			return m, nil
		}

		info, err := os.Stat(symlinkPath)
		if err != nil {
			return m, nil
		}
		if info.IsDir() {
			isDir = true
		}
	}

	if !isDir {
		return m, nil
	}

	m.CurrentDirectory = filepath.Join(m.CurrentDirectory, f.Name())
	m.pushView(m.selected, m.min, m.max)

	m.selected = 0
	m.min = 0
	m.max = m.Height - 1

	return m, m.readDir(m.CurrentDirectory)
}

// Select selects the currently focused file or directory.
func (m *Model) Select() {
	if len(m.files) == 0 {
		return
	}

	f := m.files[m.selected]
	info, err := f.Info()
	if err != nil {
		return
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0
	isDir := f.IsDir()

	filePath := filepath.Join(m.CurrentDirectory, f.Name())

	if isSymlink {
		// For symlinks, we need to resolve them using the actual filesystem.
		actualPath := filepath.Join(m.rootPath, filePath)
		symlinkPath, err := filepath.EvalSymlinks(actualPath)
		if err != nil {
			return
		}

		info, err := os.Stat(symlinkPath)
		if err != nil {
			return
		}
		if info.IsDir() {
			isDir = true
		}
	}

	if (!isDir && m.FileAllowed) || (isDir && m.DirAllowed) {
		// Set the current path as the selection.
		m.Path = filepath.Join(m.rootPath, filePath)
	}
}

// View returns the view of the file picker.
func (m Model) View() string {
	if len(m.files) == 0 {
		return m.Styles.EmptyDirectory.Height(m.Height).MaxHeight(m.Height).String()
	}

	var s strings.Builder

	for i, f := range m.files {
		if i < m.min || i > m.max {
			continue
		}

		var symlinkPath string

		info, err := f.Info()
		if err != nil {
			break
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0
		size := strings.Replace(humanize.Bytes(uint64(max(0, info.Size()))), " ", "", 1) //nolint:gosec // Uses max.
		name := f.Name()

		if isSymlink {
			// For symlinks, resolve using the actual filesystem path.
			actualPath := filepath.Join(m.rootPath, m.CurrentDirectory, name)
			symlinkPath, err = filepath.EvalSymlinks(actualPath)
			if err != nil {
				break
			}
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

	for i := lipgloss.Height(s.String()); i <= m.Height; i++ {
		s.WriteRune('\n')
	}

	contentWidth := lipgloss.Width(s.String())
	padding := max(0, m.Width-contentWidth)

	return lipgloss.NewStyle().MaxWidth(m.Width).PaddingRight(padding).Render(s.String())
}

// DidSelectFile returns whether a user has selected a file (on this msg).
func (m Model) DidSelectFile(msg tea.Msg) (bool, string) {
	didSelect, path := m.didSelectFile(msg)
	if didSelect && m.canSelect(path) {
		return true, path
	}

	return false, ""
}

// DidSelectDisabledFile returns whether a user tried to select a disabled file
// (on this msg). This is necessary only if you would like to warn the user that
// they tried to select a disabled file.
func (m Model) DidSelectDisabledFile(msg tea.Msg) (bool, string) {
	didSelect, path := m.didSelectFile(msg)
	if didSelect && !m.canSelect(path) {
		return true, path
	}

	return false, ""
}

//nolint:revive // TODO: Rename.
func (m Model) didSelectFile(msg tea.Msg) (bool, string) {
	if len(m.files) == 0 {
		return false, ""
	}

	switch msg.(type) {
	case tea.KeyMsg:
		// The key press was a selection, let's confirm whether the current file could
		// be selected or used for navigating deeper into the stack.
		f := m.files[m.selected]
		info, err := f.Info()
		if err != nil {
			return false, ""
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0
		isDir := f.IsDir()

		if isSymlink {
			// For symlinks, resolve using the actual filesystem path.
			actualPath := filepath.Join(m.rootPath, m.CurrentDirectory, f.Name())
			symlinkPath, err := filepath.EvalSymlinks(actualPath)
			if err != nil {
				break
			}

			info, err := os.Stat(symlinkPath)
			if err != nil {
				break
			}
			if info.IsDir() {
				isDir = true
			}
		}

		if (!isDir && m.FileAllowed) || (isDir && m.DirAllowed) && m.Path != "" {
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
