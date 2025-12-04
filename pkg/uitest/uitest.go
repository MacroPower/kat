package uitest

import (
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"

	tea "github.com/charmbracelet/bubbletea"
)

// BubbleModel is a constraint for Bubble Tea model types that return their
// concrete type from Update instead of [tea.Model].
type BubbleModel[T any] interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (T, tea.Cmd) //nolint:ireturn // Must satisfy [tea.Model].
	View() string
}

// modelAdapter wraps a concrete model type to satisfy [tea.Model].
type modelAdapter[T BubbleModel[T]] struct {
	model T
}

func (a modelAdapter[T]) Init() tea.Cmd {
	return a.model.Init()
}

//nolint:ireturn // Must satisfy [tea.Model].
func (a modelAdapter[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := a.model.Update(msg)
	return modelAdapter[T]{model: m}, cmd
}

func (a modelAdapter[T]) View() string {
	return a.model.View()
}

// NewTestModel creates a new test model with the given terminal size.
// It accepts models that return their concrete type from Update.
func NewTestModel[T BubbleModel[T]](tb testing.TB, m T, size Size) *teatest.TestModel {
	tb.Helper()

	return teatest.NewTestModel(
		tb, modelAdapter[T]{model: m},
		teatest.WithInitialTermSize(size.Width, size.Height),
	)
}

// WaitFor waits for a condition to be met in the output.
func WaitFor(
	tb testing.TB,
	r io.Reader,
	condition func([]byte) bool,
	opts ...teatest.WaitForOption,
) {
	tb.Helper()
	teatest.WaitFor(tb, r, condition, opts...)
}

// WaitForCapture waits for a condition to be met and returns the output.
// This is useful for capturing intermediate states during testing.
// Since Bubble Tea renders complete views, the returned bytes contain
// the full view at the moment the condition was satisfied.
func WaitForCapture(
	tb testing.TB,
	r io.Reader,
	condition func([]byte) bool,
	opts ...teatest.WaitForOption,
) string {
	tb.Helper()

	var captured []byte

	teatest.WaitFor(tb, r, func(b []byte) bool {
		if condition(b) {
			captured = make([]byte, len(b))
			copy(captured, b)

			return true
		}

		return false
	}, opts...)

	return string(captured)
}

// GetFinalOutput reads all output after the program finishes.
func GetFinalOutput(tb testing.TB, tm *teatest.TestModel, timeout time.Duration) string {
	tb.Helper()

	return string(readAll(tb, tm.FinalOutput(tb, teatest.WithFinalTimeout(timeout))))
}

func readAll(tb testing.TB, r io.Reader) []byte {
	tb.Helper()

	b, err := io.ReadAll(r)
	if err != nil {
		tb.Fatal(err)
	}

	return b
}
