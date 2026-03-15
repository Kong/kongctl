package theme

import (
	"testing"

	tint "github.com/lrstanley/bubbletint/v2"
	"github.com/stretchr/testify/require"
)

func TestColorAdaptivePreservesLightDarkVariants(t *testing.T) {
	prev := hasDarkBackground
	t.Cleanup(func() {
		hasDarkBackground = prev
	})

	c := Color{
		Light: "#112233",
		Dark:  "#AABBCC",
	}

	hasDarkBackground = func() bool { return false }
	lightR, lightG, lightB, _ := c.Adaptive().RGBA()

	hasDarkBackground = func() bool { return true }
	darkR, darkG, darkB, _ := c.Adaptive().RGBA()

	require.NotEqual(t, lightR, darkR)
	require.NotEqual(t, lightG, darkG)
	require.NotEqual(t, lightB, darkB)
}

func TestColorAdaptiveFallbackRemainsContrastSafe(t *testing.T) {
	prev := hasDarkBackground
	t.Cleanup(func() {
		hasDarkBackground = prev
	})

	hasDarkBackground = func() bool { return false }
	lightR, lightG, lightB, _ := (Color{}).Adaptive().RGBA()

	hasDarkBackground = func() bool { return true }
	darkR, darkG, darkB, _ := (Color{}).Adaptive().RGBA()

	require.Equal(t, uint32(0xFFFF), lightR)
	require.Equal(t, uint32(0xFFFF), lightG)
	require.Equal(t, uint32(0xFFFF), lightB)
	require.Equal(t, uint32(0x0000), darkR)
	require.Equal(t, uint32(0x0000), darkG)
	require.Equal(t, uint32(0x0000), darkB)
}

func TestPaletteFromTintPreservesAbout(t *testing.T) {
	p := paletteFromTint(&tint.Tint{
		ID:          "example",
		DisplayName: "Example Theme",
		CreditSources: []*tint.CreditSource{
			{Name: "Alice", Link: "https://example.com"},
		},
	})

	require.Equal(t, "Tint: Example Theme\nTint credits:\n  * Alice (https://example.com)", p.About)
}
