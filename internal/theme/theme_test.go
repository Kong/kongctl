package theme

import (
	"sync"
	"testing"

	tint "github.com/lrstanley/bubbletint/v2"
	"github.com/lucasb-eyer/go-colorful"
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

func TestKongDarkSelectedRowColorsHaveReadableContrast(t *testing.T) {
	p := kongDarkPalette()
	selection, err := colorful.Hex(p.Color(ColorSelection).Dark)
	require.NoError(t, err)
	text, err := colorful.Hex(p.Color(ColorSelectionText).Dark)
	require.NoError(t, err)

	require.Equal(t, "#CCFF00", p.Color(ColorSelection).Dark)
	require.Equal(t, "#000F06", p.Color(ColorSelectionText).Dark)
	require.GreaterOrEqual(t, contrastRatio(selection, text), 7.0)
}

func TestKongLightAccentTextHasReadableContrast(t *testing.T) {
	p := kongLightPalette()
	surface, err := colorful.Hex(p.Color(ColorSurface).Light)
	require.NoError(t, err)
	accent, err := colorful.Hex(p.Color(ColorAccent).Light)
	require.NoError(t, err)

	require.GreaterOrEqual(t, contrastRatio(surface, accent), 7.0)
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
	prev := termenvHasDarkBackground
	t.Cleanup(func() {
		termenvHasDarkBackground = prev
	})
	termenvHasDarkBackground = func() bool { return false }

	t.Setenv("COLORFGBG", "15;0")
	require.True(t, detectDarkBackgroundFromEnv())

	t.Setenv("COLORFGBG", "bogus")
	require.False(t, detectDarkBackgroundFromEnv())
}

func TestDetectDarkBackgroundFallsBackToTermenv(t *testing.T) {
	prev := termenvHasDarkBackground
	t.Cleanup(func() {
		termenvHasDarkBackground = prev
	})
	termenvHasDarkBackground = func() bool { return true }

	t.Setenv("COLORFGBG", "")
	require.True(t, detectDarkBackgroundFromEnv())
}

func TestDetectDarkBackgroundMemoizesEnv(t *testing.T) {
	darkBackgroundOnce = sync.Once{}
	darkBackgroundCached = false
	t.Cleanup(func() {
		darkBackgroundOnce = sync.Once{}
		darkBackgroundCached = false
	})

	t.Setenv("COLORFGBG", "15;0")
	require.True(t, detectDarkBackground())

	t.Setenv("COLORFGBG", "0;15")
	require.True(t, detectDarkBackground())
}

func TestSetCurrentAutoUsesDetectedBackground(t *testing.T) {
	prev := hasDarkBackground
	t.Cleanup(func() {
		hasDarkBackground = prev
		require.NoError(t, SetCurrent(DefaultName))
	})

	hasDarkBackground = func() bool { return true }
	require.NoError(t, SetCurrent(AutoName))
	require.Equal(t, DarkName, CurrentName())

	hasDarkBackground = func() bool { return false }
	require.NoError(t, SetCurrent(AutoName))
	require.Equal(t, DefaultName, CurrentName())
}

func TestThemeFlagAcceptsAuto(t *testing.T) {
	flag := NewFlag(AutoName)
	require.Equal(t, AutoName, flag.String())

	require.NoError(t, flag.Set(""))
	require.Equal(t, AutoName, flag.String())

	require.NoError(t, flag.Set(AutoName))
	require.Equal(t, AutoName, flag.String())
}

func contrastRatio(a, b colorful.Color) float64 {
	l1 := relativeLuminance(a)
	l2 := relativeLuminance(b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}
