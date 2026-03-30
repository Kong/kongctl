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

func TestDarkBackgroundFromColorFGBG(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		dark  bool
		known bool
	}{
		{name: "dark background", value: "15;0", dark: true, known: true},
		{name: "light background", value: "0;15", dark: false, known: true},
		{name: "bright black background", value: "7;8", dark: true, known: true},
		{name: "standalone light background", value: "7", dark: false, known: true},
		{name: "empty", value: "", dark: false, known: false},
		{name: "invalid", value: "bogus", dark: false, known: false},
		{name: "empty background token", value: "7;", dark: false, known: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dark, known := darkBackgroundFromColorFGBG(tt.value)
			require.Equal(t, tt.dark, dark)
			require.Equal(t, tt.known, known)
		})
	}
}

func TestDetectDarkBackgroundFromEnv(t *testing.T) {
	t.Setenv("COLORFGBG", "15;0")
	require.True(t, detectDarkBackgroundFromEnv())

	t.Setenv("COLORFGBG", "bogus")
	require.False(t, detectDarkBackgroundFromEnv())
}
