package theme

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"go.jacobcolvin.com/niceyaml/style"
)

func HuhTheme(t *Theme) huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		h := huh.ThemeBase(isDark)

		h.Focused.Base = h.Focused.Base.BorderForeground(t.Style(style.TextAccent).GetForeground())
		h.Focused.Card = h.Focused.Base
		h.Focused.Title = h.Focused.Title.Foreground(t.Style(style.TextAccent).GetForeground()).Bold(true)
		h.Focused.NoteTitle = h.Focused.NoteTitle.
			Foreground(t.Style(style.TextAccent).GetForeground()).
			Bold(true).
			MarginBottom(1)
		h.Focused.Directory = h.Focused.Directory.Foreground(t.Style(style.TextAccentDim).GetForeground())
		h.Focused.Description = h.Focused.Description.Foreground(t.Style(style.TextAccentDim).GetForeground())
		h.Focused.ErrorIndicator = h.Focused.ErrorIndicator.Foreground(t.Style(style.TextError).GetForeground())
		h.Focused.ErrorMessage = h.Focused.ErrorMessage.Foreground(t.Style(style.TextError).GetForeground())
		h.Focused.SelectSelector = h.Focused.SelectSelector.Foreground(t.Style(style.TextAccent).GetForeground())
		h.Focused.NextIndicator = h.Focused.NextIndicator.Foreground(t.Style(style.TextAccent).GetForeground())
		h.Focused.PrevIndicator = h.Focused.PrevIndicator.Foreground(t.Style(style.TextAccent).GetForeground())
		h.Focused.Option = h.Focused.Option.Foreground(t.Style(style.Text).GetBackground())
		h.Focused.MultiSelectSelector = h.Focused.MultiSelectSelector.Foreground(
			t.Style(style.TextAccent).GetForeground(),
		)
		h.Focused.SelectedOption = h.Focused.SelectedOption.Foreground(t.Style(style.TextAccent).GetForeground())
		h.Focused.SelectedPrefix = lipgloss.NewStyle().
			Foreground(t.Style(style.TextAccent).GetForeground()).
			SetString("✓ ")
		h.Focused.UnselectedPrefix = lipgloss.NewStyle().
			Foreground(t.Style(style.TextSubtleDim).GetForeground()).
			SetString("• ")
		h.Focused.UnselectedOption = h.Focused.UnselectedOption.
			Foreground(t.Style(style.Text).GetBackground())
		h.Focused.FocusedButton = h.Focused.FocusedButton.
			Foreground(t.Style(style.Title).GetForeground()).
			Background(t.Style(style.Title).GetBackground())
		h.Focused.Next = h.Focused.FocusedButton
		h.Focused.BlurredButton = h.Focused.BlurredButton.
			Foreground(t.Style(style.Title).GetForeground()).
			Background(t.Style(style.TextSubtleDim).GetForeground())

		h.Focused.TextInput.Cursor = h.Focused.TextInput.Cursor.Foreground(t.Style(style.TextAccent).GetForeground())
		h.Focused.TextInput.Placeholder = h.Focused.TextInput.Placeholder.Foreground(
			t.Style(style.TextSubtleDim).GetForeground(),
		)
		h.Focused.TextInput.Prompt = h.Focused.TextInput.Prompt.Foreground(t.Style(style.TextAccent).GetForeground())

		h.Blurred = h.Focused
		h.Blurred.Base = h.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
		h.Blurred.Card = h.Blurred.Base
		h.Blurred.NextIndicator = lipgloss.NewStyle()
		h.Blurred.PrevIndicator = lipgloss.NewStyle()

		h.Group.Title = h.Focused.Title
		h.Group.Description = h.Focused.Description

		return h
	})
}
