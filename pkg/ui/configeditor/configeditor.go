package configeditor

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-shellwords"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/command"
	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/filepicker"
	"github.com/macropower/kat/pkg/ui/theme"
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
	FindProfiles(path string) ([]command.ProfileMatch, error)
	FS() (*command.FilteredFS, error)
}

func NewModel(cmd Commander, t *theme.Theme) Model {
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

	m.form = huh.NewForm(
		huh.NewGroup(
			NewFilePicker(filepicker.New(fsys)).
				Key("file").
				Picking(true).
				Title("Select a file or directory").
				ShowPermissions(true).
				ShowSize(true).
				DirAllowed(true).
				FileAllowed(true).
				Validate(func(s string) error {
					_, err := cmd.FindProfiles(s)
					if err != nil {
						return fmt.Errorf("path error: %w", err)
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewText().
				Key("extraArgs").
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

			huh.NewConfirm().
				Key("confirm").
				Title("Ready?").
				Affirmative("Render").
				Negative(""),
		),
	).
		WithLayout(huh.LayoutGrid(1, 2)).
		WithShowHelp(false).
		WithTheme(ThemeToHuhTheme(t))

	return m
}

func ThemeToHuhTheme(t *theme.Theme) *huh.Theme {
	h := huh.ThemeBase()

	h.Focused.Base = h.Focused.Base.BorderForeground(t.SelectedStyle.GetForeground())
	h.Focused.Card = h.Focused.Base
	h.Focused.Title = h.Focused.Title.Foreground(t.SelectedStyle.GetForeground()).Bold(true)
	h.Focused.NoteTitle = h.Focused.NoteTitle.Foreground(t.SelectedStyle.GetForeground()).Bold(true).MarginBottom(1)
	h.Focused.Directory = h.Focused.Directory.Foreground(t.SelectedSubtleStyle.GetForeground())
	h.Focused.Description = h.Focused.Description.Foreground(t.SelectedSubtleStyle.GetForeground())
	h.Focused.ErrorIndicator = h.Focused.ErrorIndicator.Foreground(t.ErrorTextStyle.GetForeground())
	h.Focused.ErrorMessage = h.Focused.ErrorMessage.Foreground(t.ErrorTextStyle.GetForeground())
	h.Focused.SelectSelector = h.Focused.SelectSelector.Foreground(t.SelectedStyle.GetForeground())
	h.Focused.NextIndicator = h.Focused.NextIndicator.Foreground(t.SelectedStyle.GetForeground())
	h.Focused.PrevIndicator = h.Focused.PrevIndicator.Foreground(t.SelectedStyle.GetForeground())
	h.Focused.Option = h.Focused.Option.Foreground(t.GenericTextStyle.GetBackground())
	h.Focused.MultiSelectSelector = h.Focused.MultiSelectSelector.Foreground(t.SelectedStyle.GetForeground())
	h.Focused.SelectedOption = h.Focused.SelectedOption.Foreground(t.SelectedStyle.GetForeground())
	h.Focused.SelectedPrefix = lipgloss.NewStyle().
		Foreground(t.SelectedStyle.GetForeground()).
		SetString("✓ ")
	h.Focused.UnselectedPrefix = lipgloss.NewStyle().
		Foreground(t.SubtleStyle.GetForeground()).
		SetString("• ")
	h.Focused.UnselectedOption = h.Focused.UnselectedOption.
		Foreground(t.GenericTextStyle.GetBackground())
	h.Focused.FocusedButton = h.Focused.FocusedButton.
		Foreground(t.LogoStyle.GetForeground()).
		Background(t.LogoStyle.GetBackground())
	h.Focused.Next = h.Focused.FocusedButton
	h.Focused.BlurredButton = h.Focused.BlurredButton.
		Foreground(t.LogoStyle.GetForeground()).
		Background(t.SubtleStyle.GetForeground())

	h.Focused.TextInput.Cursor = h.Focused.TextInput.Cursor.Foreground(t.SelectedStyle.GetForeground())
	h.Focused.TextInput.Placeholder = h.Focused.TextInput.Placeholder.Foreground(t.SubtleStyle.GetForeground())
	h.Focused.TextInput.Prompt = h.Focused.TextInput.Prompt.Foreground(t.SelectedStyle.GetForeground())

	h.Blurred = h.Focused
	h.Blurred.Base = h.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	h.Blurred.Card = h.Blurred.Base
	h.Blurred.NextIndicator = lipgloss.NewStyle()
	h.Blurred.PrevIndicator = lipgloss.NewStyle()

	h.Group.Title = h.Focused.Title
	h.Group.Description = h.Focused.Description

	return h
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
		case "file":
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

func (m Model) Result() Result {
	argStr := m.form.GetString("extraArgs")
	extraArgs, err := shellwords.Parse(argStr)
	if err != nil {
		slog.Error("skipping extra arguments",
			slog.Any("error", err),
		)
	}

	return Result{
		File:      m.form.GetString("file"),
		Profile:   *m.selectedProfileName,
		ExtraArgs: extraArgs,
	}
}

func (m Model) View() string {
	if m.form.State == huh.StateCompleted {
		return ""
	}

	return lipgloss.NewStyle().
		Height(m.height).
		Render(m.form.View())
}

func (m *Model) SetHeight(h int) {
	m.form.WithHeight(h - 2)

	m.height = h
}
