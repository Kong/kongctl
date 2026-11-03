package theme

import "context"

type fallbackContextKey struct{}

// ContextWithFallback records the removed theme that caused automatic selection.
func ContextWithFallback(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, fallbackContextKey{}, name)
}

// FallbackFromContext returns the removed theme that caused automatic selection.
func FallbackFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	name, _ := ctx.Value(fallbackContextKey{}).(string)
	return name
}

// legacyTintNames records IDs lost when bubbletint switched catalogs in v2.0.3.
// Replacements preserve all colors used by paletteFromTint. Empty replacements
// identify removed themes without a verified equivalent.
var legacyTintNames = map[string]string{
	"bubbles":                           "",
	"builtin_solarized_dark":            "",
	"builtin_solarized_light":           "",
	"contrast_light":                    "",
	"coolnight":                         "cool_night",
	"crystal_violet":                    "",
	"cyber_cube":                        "",
	"cyber_punk_2077":                   "",
	"ganyu":                             "",
	"github":                            "",
	"glorious":                          "",
	"h_4_rithd":                         "",
	"h_4_rithd_com":                     "",
	"juicy_colors":                      "",
	"kanagawa":                          "",
	"liquid_carbon_transparent_inverse": "",
	"monokai_cmder":                     "",
	"monokai_pro_filter_octagon":        "",
	"monokai_pro_filter_ristretto":      "",
	"moonlight_ii":                      "",
	"one_dark":                          "",
	"one_star":                          "",
	"primer":                            "",
	"qb_64_super_dark_blue":             "",
	"retrowave":                         "",
	"solarized_dark___patched":          "solarized_dark_patched",
	"sonoran_gothic":                    "",
	"sonoran_sunrise":                   "",
	"space_gray":                        "",
	"space_gray_eighties":               "spacegray_eighties",
	"space_gray_eighties_dull":          "spacegray_eighties_dull",
	"tokyo_night_light":                 "",
	"tokyonight":                        "tokyo_night",
	"tokyonight_day":                    "tokyo_night_day",
	"tokyonight_storm":                  "",
	"unholy":                            "",
	"zeonica":                           "",
}

// IsRemoved reports whether name is an old tint ID without a verified replacement.
func IsRemoved(name string) bool {
	replacement, known := legacyTintNames[sanitizeName(name)]
	return known && replacement == ""
}
