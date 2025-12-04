package uitest

// Standard terminal sizes for testing.
const (
	// Compact sizes (80x24 - classic terminal).
	CompactWidth  = 80
	CompactHeight = 24

	// Standard sizes (120x40 - typical modern terminal).
	StandardWidth  = 120
	StandardHeight = 40

	// Wide sizes (160x50 - large display).
	WideWidth  = 160
	WideHeight = 50
)

// Size represents terminal dimensions.
type Size struct {
	Width  int
	Height int
}

// Predefined terminal sizes for consistent testing.
var (
	Compact  = Size{CompactWidth, CompactHeight}
	Standard = Size{StandardWidth, StandardHeight}
	Wide     = Size{WideWidth, WideHeight}
)
