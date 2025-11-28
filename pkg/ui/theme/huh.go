package theme

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

func HuhTheme(t *Theme) *huh.Theme {
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
