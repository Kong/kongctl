package theme

import (
	"strings"
	"testing"

	tint "github.com/lrstanley/bubbletint/v2"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/stretchr/testify/require"
)

func TestLegacyTintAliases(t *testing.T) {
	previous := CurrentName()
	t.Cleanup(func() { require.NoError(t, SetCurrent(previous)) })

	for _, tt := range []struct {
		old, replacement string
	}{
		{"coolnight", "cool_night"},
		{"solarized_dark___patched", "solarized_dark_patched"},
		{"space_gray_eighties", "spacegray_eighties"},
		{"space_gray_eighties_dull", "spacegray_eighties_dull"},
		{"tokyonight", "tokyo_night"},
		{"tokyonight_day", "tokyo_night_day"},
	} {
		t.Run(tt.old, func(t *testing.T) {
			name := "  " + tt.old + "  "
			require.True(t, Exists(name))
			require.False(t, IsRemoved(name))
			palette, ok := Get(name)
			require.True(t, ok)
			require.Equal(t, tt.replacement, palette.Name)
			require.NoError(t, SetCurrent(name))
			require.Equal(t, tt.replacement, CurrentName())
			flag := NewFlag(AutoName)
			require.NoError(t, flag.Set(name))
			require.Equal(t, tt.replacement, flag.Value())
			require.NotContains(t, Available(), tt.old)
		})
	}
}

func TestLegacyTintAliasesPreserveColors(t *testing.T) {
	// Inputs from bubbletint v2.0.2, ordered as fg, bg, bright black, cyan,
	// bright blue, green, blue, yellow, red, and bright white. These are all
	// tint fields used by paletteFromTint.
	for _, tt := range []struct {
		name, colors string
	}{
		{
			"coolnight",
			"#ECDEF3 #010C18 #63686D #FF5ED3 #388EFF #51FFD0 #1376F9 #FFF383 #FF3A3A #60FBBF",
		},
		{
			"solarized_dark___patched",
			"#708283 #001E27 #475B61 #249286 #708283 #738A05 #2076C7 #A57706 #D11C24 #FCF3DC",
		},
		{
			"space_gray_eighties",
			"#BDBAAE #222222 #555555 #57C2C1 #4D83D1 #81A764 #5486C0 #FEC254 #EC5F67 #FFFFFF",
		},
		{
			"space_gray_eighties_dull",
			"#C9C6BC #222222 #555555 #80CDCB #5486C0 #92B377 #7C8FA5 #C67359 #B24956 #FFFFFF",
		},
		{
			"tokyonight",
			"#C0CAF5 #1A1B26 #414868 #7DCFFF #79A2F7 #9ECE69 #79A2F7 #E0AF68 #F7768E #C0CAF5",
		},
		{
			"tokyonight_day",
			"#3760BF #E1E2E7 #A1A6C5 #007197 #2E7DE9 #587538 #2E7DE9 #8C6C3E #F52A65 #3760BF",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var original tint.Tint
			fields := []**tint.Color{
				&original.Fg, &original.Bg, &original.BrightBlack, &original.Cyan, &original.BrightBlue,
				&original.Green, &original.Blue, &original.Yellow, &original.Red, &original.BrightWhite,
			}
			hexes := strings.Fields(tt.colors)
			require.Len(t, hexes, len(fields))
			for i, hex := range hexes {
				c, err := colorful.Hex(hex)
				require.NoError(t, err)
				r, g, b := c.RGB255()
				*fields[i] = &tint.Color{R: r, G: g, B: b, A: 255}
			}
			current, ok := Get(tt.name)
			require.True(t, ok)
			require.Equal(t, paletteFromTint(&original).Colors, current.Colors)
		})
	}
}

func TestRemovedTintNames(t *testing.T) {
	for name, replacement := range legacyTintNames {
		if replacement != "" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			require.True(t, IsRemoved(name))
			require.False(t, Exists(name))
			_, ok := Get(name)
			require.False(t, ok)
			flag := NewFlag(AutoName)
			require.NoError(t, flag.Set(name))
			require.Equal(t, name, flag.Value())
		})
	}
	require.True(t, IsRemoved(" KANAGAWA "))
	require.False(t, IsRemoved("kanagwa"))
	require.False(t, IsRemoved(AutoName))
	flag := NewFlag(AutoName)
	require.ErrorContains(t, flag.Set("kanagwa"), "kongctl list themes --color-theme auto")
	require.ErrorContains(t, SetCurrent("kanagwa"), "kongctl list themes --color-theme auto")
}
