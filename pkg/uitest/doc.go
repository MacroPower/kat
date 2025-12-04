// Package uitest provides testing utilities for Bubble Tea TUI components.
//
// This package enables developers (and agents) to test and debug TUI elements,
// especially ANSI styles. It combines:
//
//   - ANSI style verification: Parse and verify specific ANSI sequences
//   - Generic model adapter: Test Bubble Tea models that return concrete types
//   - Helper functions: Common assertions for TUI output
//
// # Testing Models
//
// [NewTestModel] accepts any model satisfying [BubbleModel], including models
// whose Update method returns the concrete type instead of [tea.Model]:
//
//	func TestMyComponent(t *testing.T) {
//	    t.Parallel()
//	    uitest.SetupColorProfile()
//
//	    model := NewMyModel()
//	    tm := uitest.NewTestModel(t, model, uitest.StandardSize)
//
//	    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
//	    output := uitest.GetFinalOutput(t, tm, time.Second)
//	}
//
// # Verifying Styles
//
//	func TestMyComponent(t *testing.T) {
//	    t.Parallel()
//	    uitest.SetupColorProfile()
//
//	    model := NewMyModel()
//	    output := model.View() // Alternatively, use a TestModel.
//
//	    verifier := uitest.NewANSIStyleVerifier(output)
//	    verifier.ContainsStyledText(t, "Hello", uitest.StyleExpectation{
//	        Foreground: uitest.Ptr("212"),
//	        Bold:       uitest.Ptr(true),
//	    })
//	}
package uitest
