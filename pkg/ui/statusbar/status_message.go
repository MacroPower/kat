package statusbar

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// StatusMessageTimeout is the default duration before a status message is
// auto-cleared.
const StatusMessageTimeout = 3 * time.Second

// statusMessageTimeoutMsg is sent when a status message expires.
type statusMessageTimeoutMsg struct{ seq int }

// StatusMessageModel encapsulates a temporary status bar message with
// automatic timeout clearing. Embed or compose this into any model that
// needs transient status messages.
type StatusMessageModel struct {
	message string
	style   Style
	seq     int
	visible bool
}

// Set stores a status message and returns a [tea.Cmd] that will clear it
// after [StatusMessageTimeout].
func (m *StatusMessageModel) Set(msg string, style Style) tea.Cmd {
	m.message = msg
	m.style = style
	m.visible = true

	m.seq++
	seq := m.seq

	return tea.Tick(StatusMessageTimeout, func(time.Time) tea.Msg {
		return statusMessageTimeoutMsg{seq: seq}
	})
}

// Update handles the timeout message. It returns true if the message was
// consumed, allowing callers to skip further processing.
func (m *StatusMessageModel) Update(msg tea.Msg) bool {
	if msg, ok := msg.(statusMessageTimeoutMsg); ok {
		if msg.seq == m.seq {
			m.Clear()
		}

		return true
	}

	return false
}

// Clear removes the current status message.
func (m *StatusMessageModel) Clear() {
	m.message = ""
	m.style = StyleNormal
	m.visible = false
}

// Visible reports whether a status message is currently being shown.
func (m *StatusMessageModel) Visible() bool {
	return m.visible && m.message != ""
}

// Opt returns a [StatusBarOpt] that applies the current message to a
// [StatusBarRenderer]. If no message is set, it returns nil.
func (m *StatusMessageModel) Opt() StatusBarOpt {
	if !m.Visible() {
		return nil
	}

	return WithMessage(m.message, m.style)
}
