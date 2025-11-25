package configeditor

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-shellwords"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/filepicker"
)

const (
	FieldFile      = "file"
	FieldExtraArgs = "extraArgs"
)

type Model struct {
	form *huh.Form
	cmd  Commander

	profiles            map[string]*profile.Profile
	selectedProfileName *string
	selectedPath        *string
	height              int
}

type Result struct {
	File      string
	Profile   string
	ExtraArgs []string
}

type Config struct {
	CommonModel *common.CommonModel
}

type Commander interface {
	GetProfiles() map[string]*profile.Profile
	GetCurrentProfile() (string, *profile.Profile)
	GetPath() string
	FindProfiles(path string) ([]command.ProfileMatch, error)
	FS() (*command.FilteredFS, error)
}

func NewModel(cmd Commander, t *huh.Theme, km *huh.KeyMap) Model {
	var (
		m            Model
		selectedPath string
	)

	m.selectedPath = &selectedPath

	m.profiles = cmd.GetProfiles()

	pName, _ := cmd.GetCurrentProfile()
	m.selectedProfileName = &pName

	m.cmd = cmd

	fsys, err := cmd.FS()
	if err != nil {
		panic(err)
	}

	// Start the file picker in the parent of the current path.
	startDir := filepath.Dir(cmd.GetPath())

	m.form = huh.NewForm(
		huh.NewGroup(
			NewFilePicker(filepicker.New(fsys)).
				Key(FieldFile).
				Picking(true).
				CurrentDirectory(startDir).
				Title("Select a file or directory").
				ShowPermissions(true).
				ShowSize(true).
				DirAllowed(true).
				FileAllowed(true),

			huh.NewText().
				Key(FieldExtraArgs).
				Title("Extra Arguments").
				TitleFunc(func() string {
					if m.selectedProfileName != nil && *m.selectedProfileName != "" {
						return fmt.Sprintf("Extra Arguments (%s)", *m.selectedProfileName)
					}

					return "Extra Arguments"
				}, m.selectedProfileName).
				Lines(1).
				PlaceholderFunc(func() string {
					if m.selectedProfileName != nil && *m.selectedProfileName != "" {
						if p, ok := m.profiles[*m.selectedProfileName]; ok {
							return strings.Join(p.ExtraArgs, " ")
						}
					}

					return ""
				}, m.selectedProfileName),
		),
	).
		WithShowHelp(false).
		WithTheme(t).
		WithKeyMap(km)

	return m
}

func (m Model) Init() tea.Cmd {
	return m.form.Init()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	cmds := []tea.Cmd{}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	updateForm := false
	switch msg.(type) {
	case tea.KeyMsg:
		field := m.form.GetFocusedField()
		switch field.GetKey() {
		case FieldFile:
			filePath, ok := field.GetValue().(string)
			if !ok {
				panic("file field value is not a string")
			}
			if filePath == "" {
				break
			}

			profiles, err := m.cmd.FindProfiles(filePath)
			if err != nil {
				slog.Error("error finding profiles",
					slog.Any("error", err),
				)

				*m.selectedProfileName = ""

				break
			}

			matchedProfile := profiles[0]

			*m.selectedPath = filePath
			*m.selectedProfileName = matchedProfile.Name

			updateForm = true
		}
	}

	if updateForm {
		form, cmd2 := m.form.Update(nil)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		cmds = append(cmds, tea.Sequence(cmd, cmd2))
	} else {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) IsCompleted() bool {
	return m.form.State == huh.StateCompleted
}

func (m Model) Focused() bool {
	return m.form.GetFocusedField().GetKey() == FieldExtraArgs
}

func (m Model) Result() Result {
	argStr := m.form.GetString(FieldExtraArgs)
	extraArgs, err := shellwords.Parse(argStr)
	if err != nil {
		slog.Error("skipping extra arguments",
			slog.Any("error", err),
		)
	}

	return Result{
		File:      m.form.GetString(FieldFile),
		Profile:   *m.selectedProfileName,
		ExtraArgs: extraArgs,
	}
}

func (m Model) View() string {
	if m.form.State == huh.StateCompleted {
		return lipgloss.NewStyle().
			Height(m.height).
			Render("Loading...")
	}

	return lipgloss.NewStyle().
		Height(m.height).
		Render(m.form.View())
}

func (m *Model) SetHeight(h int) {
	m.form.WithHeight(h - 2)

	m.height = h
}
