package configeditor

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss/v2"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kat/pkg/profile"
	"github.com/macropower/kat/pkg/ui/common"
	"github.com/macropower/kat/pkg/ui/filepicker"
)

type Model struct {
	form *huh.Form

	cm                  *common.CommonModel
	profiles            map[string]*profile.Profile
	selectedProfile     *profile.Profile
	selectedProfileName *string
	height              int
}

type Config struct {
	CommonModel *common.CommonModel
}

func NewModel(cfg *Config) Model {
	m := Model{
		cm: cfg.CommonModel,
	}
	m.profiles = m.cm.Cmd.GetProfiles()
	pn, p := m.cm.Cmd.GetCurrentProfile()
	m.selectedProfileName = &pn
	m.selectedProfile = p

	profileOptions := []huh.Option[string]{}
	for p := range m.profiles {
		profileOptions = append(profileOptions, huh.NewOption(p, p))
	}

	slices.SortFunc(profileOptions, func(a, b huh.Option[string]) int {
		return strings.Compare(a.Value, b.Value)
	})

	fsys, err := cfg.CommonModel.Cmd.FS()
	if err != nil {
		panic(err)
	}

	m.form = huh.NewForm(
		huh.NewGroup(
			NewFilePicker(filepicker.New(fsys, cfg.CommonModel.Theme)).
				Picking(true).
				Title("Select a file or directory").
				ShowPermissions(true).ShowSize(true).
				DirAllowed(true).
				FileAllowed(true),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Key("profile").
				Options(profileOptions...).
				Title("Choose a profile"),

			huh.NewText().
				Key("extraArgs").
				Title("Extra Arguments").
				Lines(1).
				PlaceholderFunc(func() string {
					slog.Debug("Getting extra args placeholder", slog.String("profile", *m.selectedProfileName))
					if m.selectedProfile != nil && len(m.selectedProfile.ExtraArgs) > 0 {
						return strings.Join(m.selectedProfile.ExtraArgs, " ")
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
		WithShowHelp(false)

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
		case "profile":
			profileName, ok := field.GetValue().(string)
			if !ok {
				break
			}

			profilePtr, ok := m.profiles[profileName]
			if !ok {
				break
			}

			p := *profilePtr
			*m.selectedProfileName = profileName
			*m.selectedProfile = p

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

	if m.form.State == huh.StateCompleted {
		profileName := *m.selectedProfileName
		slog.Debug("profile selected", slog.String("name", profileName))

		_ = m.cm.Cmd.SetProfile(profileName)
	}

	return m, tea.Batch(cmds...)
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
