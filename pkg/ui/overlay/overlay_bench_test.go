package overlay_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/macropower/kat/pkg/ui/overlay"
	"github.com/macropower/kat/pkg/ui/themes"
)

// Benchmark data to test different scenarios.
var (
	smallBG  = strings.Repeat("background line\n", 10)
	mediumBG = strings.Repeat("this is a longer background line with more content\n", 50)
	largeBG  = strings.Repeat("this is a very long background line with lots of content that will test performance\n", 200)

	smallFG  = "overlay content"
	mediumFG = strings.Repeat("overlay line\n", 20)
	largeFG  = strings.Repeat("this is a longer overlay line with more content\n", 100)
)

func BenchmarkOverlay_Place_Small(b *testing.B) {
	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(80, 24)
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	b.ResetTimer()
	for range b.N {
		_ = o.Place(smallBG, smallFG, 0.5, style)
	}
}

func BenchmarkOverlay_Place_Medium(b *testing.B) {
	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(120, 40)
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1)

	b.ResetTimer()
	for range b.N {
		_ = o.Place(mediumBG, mediumFG, 0.6, style)
	}
}

func BenchmarkOverlay_Place_Large(b *testing.B) {
	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(200, 80)
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(2)

	b.ResetTimer()
	for range b.N {
		_ = o.Place(largeBG, largeFG, 0.7, style)
	}
}

func BenchmarkOverlay_Place_TruncatedContent(b *testing.B) {
	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(80, 10) // Small height to force truncation
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	// Large foreground content that will be truncated
	largeFGForTruncation := strings.Repeat("this line will be truncated\n", 50)

	b.ResetTimer()
	for range b.N {
		_ = o.Place(mediumBG, largeFGForTruncation, 0.8, style)
	}
}

func BenchmarkOverlay_Place_WideContent(b *testing.B) {
	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(300, 50)
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	// Wide content to test horizontal performance
	wideBG := strings.Repeat(strings.Repeat("X", 280)+"\n", 40)
	wideFG := strings.Repeat(strings.Repeat("O", 100)+"\n", 20)

	b.ResetTimer()
	for range b.N {
		_ = o.Place(wideBG, wideFG, 0.4, style)
	}
}

func BenchmarkOverlay_Place_MinimalWidth(b *testing.B) {
	theme := themes.DefaultTheme
	o := overlay.New(theme, overlay.WithMinWidth(32))
	o.SetSize(80, 24)
	style := lipgloss.NewStyle()

	b.ResetTimer()
	for range b.N {
		_ = o.Place(smallBG, smallFG, 0.1, style) // Small fraction to test min width
	}
}

// Benchmark memory allocations specifically
func BenchmarkOverlay_Place_Allocations(b *testing.B) {
	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(120, 40)
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = o.Place(mediumBG, mediumFG, 0.5, style)
	}
}

// Benchmark with Unicode content to test character width calculations
func BenchmarkOverlay_Place_Unicode(b *testing.B) {
	theme := themes.DefaultTheme
	o := overlay.New(theme)
	o.SetSize(100, 30)
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	unicodeBG := strings.Repeat("背景内容 with 混合 content 🚀\n", 25)
	unicodeFG := strings.Repeat("覆盖层内容 overlay 🎯\n", 15)

	b.ResetTimer()
	for range b.N {
		_ = o.Place(unicodeBG, unicodeFG, 0.6, style)
	}
}
