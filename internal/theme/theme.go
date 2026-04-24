package theme

import (
	"context"
	"fmt"
	"image/color"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
	"github.com/lucasb-eyer/go-colorful"
)

// DefaultName is the built-in theme used when no override is provided.
const DefaultName = "kong-light"

// LegacyName is the deprecated theme name kept for backward compatibility.
const LegacyName = "kong"

// Token represents a semantic color slot within the CLI.
type Token string

const (
	ColorTextPrimary   Token = "text.primary"
	ColorTextSecondary Token = "text.secondary"
	ColorTextMuted     Token = "text.muted"
	ColorBorder        Token = "border"
	ColorSurface       Token = "surface"
	ColorSurfaceText   Token = "surface.text"
	ColorPrimary       Token = "primary"
	ColorPrimaryText   Token = "primary.text"
	ColorAccent        Token = "accent"
	ColorAccentText    Token = "accent.text"
	ColorSuccess       Token = "success"
	ColorSuccessText   Token = "success.text"
	ColorInfo          Token = "info"
	ColorInfoText      Token = "info.text"
	ColorWarning       Token = "warning"
	ColorWarningText   Token = "warning.text"
	ColorDanger        Token = "danger"
	ColorDangerText    Token = "danger.text"
	ColorHighlight     Token = "highlight"
)

// Color stores light and dark variants for adaptive rendering.
type Color struct {
	Light string
	Dark  string
}

type adaptiveColor struct {
	light color.Color
	dark  color.Color
}

func (c adaptiveColor) RGBA() (uint32, uint32, uint32, uint32) {
	if hasDarkBackground() {
		return c.dark.RGBA()
	}
	return c.light.RGBA()
}

// Adaptive returns a lipgloss-compatible adaptive color that preserves
// distinct light and dark variants when both are provided.
func (c Color) Adaptive() color.Color {
	light := strings.TrimSpace(c.Light)
	dark := strings.TrimSpace(c.Dark)
	switch {
	case light == "" && dark == "":
		return adaptiveColor{
			light: lipgloss.Color("#FFFFFF"),
			dark:  lipgloss.Color("#000000"),
		}
	case light == "":
		light = dark
	case dark == "":
		dark = light
	}
	return adaptiveColor{
		light: lipgloss.Color(light),
		dark:  lipgloss.Color(dark),
	}
}

// Palette represents a concrete theme.
type Palette struct {
	Name        string
	DisplayName string
	About       string
	Colors      map[Token]Color
}

// Color returns a color for the provided token, falling back to the default palette.
func (p Palette) Color(token Token) Color {
	if p.Colors != nil {
		if c, ok := p.Colors[token]; ok {
			return ensureColor(c, token)
		}
	}
	return fallbackColor(token)
}

// Adaptive returns the resolved lipgloss color for the provided token.
func (p Palette) Adaptive(token Token) color.Color {
	return p.Color(token).Adaptive()
}

// ForegroundStyle returns a lipgloss style with the foreground set to the requested token.
func (p Palette) ForegroundStyle(token Token) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(p.Adaptive(token))
}

// BackgroundStyle returns a lipgloss style with the background set to the requested token.
func (p Palette) BackgroundStyle(token Token) lipgloss.Style {
	return lipgloss.NewStyle().Background(p.Adaptive(token))
}

type contextKey struct{}

var (
	registryOnce         sync.Once
	registryMu           sync.RWMutex
	palettes             map[string]Palette
	current              Palette
	defaultPal           Palette
	themeKey             contextKey
	configuredExplicitly bool
	darkBackgroundOnce   sync.Once
	darkBackgroundCached bool
	hasDarkBackground    = detectDarkBackground
)

func detectDarkBackground() bool {
	darkBackgroundOnce.Do(func() {
		darkBackgroundCached = detectDarkBackgroundFromEnv()
	})

	return darkBackgroundCached
}

func detectDarkBackgroundFromEnv() bool {
	dark, ok := darkBackgroundFromColorFGBG(os.Getenv("COLORFGBG"))
	return ok && dark
}

func darkBackgroundFromColorFGBG(value string) (bool, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, false
	}

	bgText := value
	if idx := strings.LastIndex(value, ";"); idx >= 0 {
		bgText = strings.TrimSpace(value[idx+1:])
		if bgText == "" {
			return false, false
		}
	}

	bg, err := strconv.Atoi(strings.TrimSpace(bgText))
	if err != nil || bg < 0 {
		return false, false
	}

	switch {
	case bg <= 6 || bg == 8:
		return true, true
	case bg == 7 || (bg >= 9 && bg <= 15):
		return false, true
	default:
		return false, false
	}
}

// ContextWithPalette stores the palette on the context.
func ContextWithPalette(ctx context.Context, p Palette) context.Context {
	return context.WithValue(ctx, themeKey, p)
}

// FromContext returns the palette stored on the context or the current palette.
func FromContext(ctx context.Context) Palette {
	if ctx == nil {
		return Current()
	}
	if p, ok := ctx.Value(themeKey).(Palette); ok {
		return p
	}
	return Current()
}

// Available returns the list of registered theme IDs (sorted).
func Available() []string {
	ensureRegistry()

	registryMu.RLock()
	defer registryMu.RUnlock()

	keys := slices.Collect(maps.Keys(palettes))
	slices.Sort(keys)
	return keys
}

// AvailableDisplayNames returns a map of theme IDs to display names.
func AvailableDisplayNames() map[string]string {
	ensureRegistry()

	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make(map[string]string, len(palettes))
	for id, p := range palettes {
		names[id] = p.DisplayName
	}
	return names
}

// Exists returns true when a theme is registered.
func Exists(name string) bool {
	ensureRegistry()

	registryMu.RLock()
	defer registryMu.RUnlock()

	_, ok := palettes[resolveName(name)]
	return ok
}

// Get returns the palette with the provided name.
func Get(name string) (Palette, bool) {
	ensureRegistry()

	registryMu.RLock()
	defer registryMu.RUnlock()

	p, ok := palettes[resolveName(name)]
	return p, ok
}

// SetCurrent sets the active palette.
func SetCurrent(name string) error {
	ensureRegistry()

	name = resolveName(name)
	if name == "" {
		name = DefaultName
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	p, ok := palettes[name]
	if !ok {
		return fmt.Errorf("unknown color theme %q", name)
	}
	current = p
	return nil
}

// SetConfiguredExplicitly records whether the active theme was set by the user
// (via config file, environment variable, or flag) rather than falling back to
// the built-in default.
func SetConfiguredExplicitly(v bool) {
	registryMu.Lock()
	defer registryMu.Unlock()
	configuredExplicitly = v
}

// IsConfiguredExplicitly reports whether the user has explicitly chosen a theme.
func IsConfiguredExplicitly() bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return configuredExplicitly
}

// Current returns the active palette.
func Current() Palette {
	ensureRegistry()

	registryMu.RLock()
	defer registryMu.RUnlock()

	return current
}

// CurrentName returns the ID of the active palette.
func CurrentName() string {
	return resolveName(Current().Name)
}

// Flag is a pflag.Value implementation for theme IDs.
type Flag struct {
	value string
}

// NewFlag returns a Flag with the provided default value.
func NewFlag(defaultValue string) *Flag {
	name := resolveName(defaultValue)
	if name == "" || !Exists(name) {
		name = DefaultName
	}
	return &Flag{value: name}
}

// String implements pflag.Value.
func (f *Flag) String() string {
	if f == nil {
		return DefaultName
	}
	return f.value
}

// Set implements pflag.Value.
func (f *Flag) Set(v string) error {
	name := resolveName(v)
	if name == "" {
		name = DefaultName
	}
	if !Exists(name) {
		return fmt.Errorf("invalid color theme %q", v)
	}
	f.value = name
	return nil
}

// Type implements pflag.Value.
func (f *Flag) Type() string {
	return "string"
}

// Value returns the currently selected theme ID.
func (f *Flag) Value() string {
	return f.String()
}

// ensureRegistry lazily loads the palettes.
func ensureRegistry() {
	registryOnce.Do(func() {
		registryMu.Lock()
		defer registryMu.Unlock()

		palettes = make(map[string]Palette)

		registerPalette(kongLightPalette())
		registerPalette(kongDarkPalette())
		defaultPal = palettes[DefaultName]
		current = defaultPal

		tint.NewDefaultRegistry()
		for _, t := range tint.Tints() {
			registerPalette(paletteFromTint(t))
		}
	})
}

func registerPalette(p Palette) {
	if p.Name == "" {
		return
	}
	if p.DisplayName == "" {
		p.DisplayName = p.Name
	}
	if p.Colors == nil {
		p.Colors = map[Token]Color{}
	}
	p.Name = sanitizeName(p.Name)
	palettes[p.Name] = p
}

func ensureColor(c Color, token Token) Color {
	if strings.TrimSpace(c.Light) == "" && strings.TrimSpace(c.Dark) == "" {
		return fallbackColor(token)
	}
	if strings.TrimSpace(c.Light) == "" {
		c.Light = c.Dark
	}
	if strings.TrimSpace(c.Dark) == "" {
		c.Dark = c.Light
	}
	return c
}

func fallbackColor(token Token) Color {
	if defaultPal.Colors != nil {
		if c, ok := defaultPal.Colors[token]; ok {
			return ensureColor(c, token)
		}
	}
	return Color{Light: "#FFFFFF", Dark: "#000000"}
}

func sanitizeName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func resolveName(name string) string {
	normalized := sanitizeName(name)
	switch normalized {
	case "":
		return ""
	case LegacyName:
		return DefaultName
	default:
		return normalized
	}
}

func tintColorHex(c *tint.Color) string {
	if c == nil {
		return ""
	}
	return normalizeHex(c.Hex())
}

func paletteFromTint(t *tint.Tint) Palette {
	if t == nil {
		return Palette{}
	}

	fg := tintColorHex(t.Fg)
	bg := tintColorHex(t.Bg)
	muted := tintColorHex(t.BrightBlack)
	accent := tintColorHex(t.Cyan)
	accentBright := tintColorHex(t.BrightBlue)
	success := tintColorHex(t.Green)
	info := tintColorHex(t.Blue)
	warning := tintColorHex(t.Yellow)
	danger := tintColorHex(t.Red)
	highlight := tintColorHex(t.BrightWhite)

	colors := map[Token]Color{
		ColorTextPrimary:   pairColor(fg, fg),
		ColorTextSecondary: derivedTextSecondary(fg),
		ColorTextMuted:     mutedColor(muted),
		ColorBorder:        borderColor(muted),
		ColorSurface:       pairColor(bg, bg),
		ColorSurfaceText:   pairColor(fg, fg),
		ColorPrimary:       pairColor(accent, accent),
		ColorPrimaryText:   singleColor(contrastColor(accent)),
		ColorAccent:        pairColor(accentBright, accentBright),
		ColorAccentText:    singleColor(contrastColor(accentBright)),
		ColorSuccess:       pairColor(success, success),
		ColorSuccessText:   singleColor(contrastColor(success)),
		ColorInfo:          pairColor(info, info),
		ColorInfoText:      singleColor(contrastColor(info)),
		ColorWarning:       pairColor(warning, warning),
		ColorWarningText:   singleColor(contrastColor(warning)),
		ColorDanger:        pairColor(danger, danger),
		ColorDangerText:    singleColor(contrastColor(danger)),
		ColorHighlight:     pairColor(highlight, highlight),
	}

	return Palette{
		Name:        sanitizeName(t.ID),
		DisplayName: strings.TrimSpace(t.DisplayName),
		About:       aboutFromTint(t),
		Colors:      colors,
	}
}

func aboutFromTint(t *tint.Tint) string {
	if t == nil {
		return ""
	}

	name := strings.TrimSpace(t.DisplayName)
	if name == "" {
		name = strings.TrimSpace(t.ID)
	}
	if name == "" {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Tint: %s", name)

	credits := make([]string, 0, len(t.CreditSources))
	for _, source := range t.CreditSources {
		if source == nil {
			continue
		}

		sourceName := strings.TrimSpace(source.Name)
		sourceLink := strings.TrimSpace(source.Link)
		switch {
		case sourceName != "" && sourceLink != "":
			credits = append(credits, fmt.Sprintf("%s (%s)", sourceName, sourceLink))
		case sourceName != "":
			credits = append(credits, sourceName)
		case sourceLink != "":
			credits = append(credits, sourceLink)
		}
	}

	if len(credits) == 0 {
		return b.String()
	}

	b.WriteString("\nTint credits:")
	for _, credit := range credits {
		fmt.Fprintf(&b, "\n  * %s", credit)
	}

	return b.String()
}

func singleColor(hex string) Color {
	h := normalizeHex(hex)
	return Color{Light: h, Dark: h}
}

func pairColor(light, dark string) Color {
	return Color{
		Light: normalizeHex(light),
		Dark:  normalizeHex(dark),
	}
}

func mutedColor(hex string) Color {
	base := normalizeHex(hex)
	if base == "" {
		return Color{Light: "#646A7A", Dark: "#7C8298"}
	}
	return Color{
		Light: darkenHex(base, 0.35),
		Dark:  lightenHex(base, 0.35),
	}
}

func derivedTextSecondary(hex string) Color {
	base := normalizeHex(hex)
	if base == "" {
		return Color{Light: "#1F2026", Dark: "#D7D9E3"}
	}
	return Color{
		Light: darkenHex(base, 0.25),
		Dark:  lightenHex(base, 0.2),
	}
}

func borderColor(hex string) Color {
	base := normalizeHex(hex)
	if base == "" {
		return Color{Light: "#4A4D65", Dark: "#4A4D65"}
	}
	return Color{
		Light: darkenHex(base, 0.15),
		Dark:  lightenHex(base, 0.25),
	}
}

func normalizeHex(hex string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(hex, "#"))
	if trimmed == "" {
		return ""
	}
	switch len(trimmed) {
	case 3:
		var b strings.Builder
		b.WriteString("#")
		for _, r := range trimmed {
			b.WriteRune(r)
			b.WriteRune(r)
		}
		return strings.ToUpper(b.String())
	case 6:
		return "#" + strings.ToUpper(trimmed)
	case 8:
		return "#" + strings.ToUpper(trimmed)
	default:
		if len(trimmed) > 6 {
			return "#" + strings.ToUpper(trimmed[:6])
		}
		return "#" + strings.ToUpper(trimmed)
	}
}

func contrastColor(hex string) string {
	h := normalizeHex(hex)
	if h == "" {
		return "#121418"
	}
	c, err := colorful.Hex(h)
	if err != nil {
		return "#121418"
	}
	if relativeLuminance(c) > 0.55 {
		return "#121418"
	}
	return "#F8F8F8"
}

func lightenHex(hex string, amount float64) string {
	h := normalizeHex(hex)
	if h == "" {
		return ""
	}
	c, err := colorful.Hex(h)
	if err != nil {
		return h
	}
	return c.BlendLab(colorful.Color{R: 1, G: 1, B: 1}, clampFloat(amount, 0, 1)).Clamped().Hex()
}

func darkenHex(hex string, amount float64) string {
	h := normalizeHex(hex)
	if h == "" {
		return ""
	}
	c, err := colorful.Hex(h)
	if err != nil {
		return h
	}
	return c.BlendLab(colorful.Color{R: 0, G: 0, B: 0}, clampFloat(amount, 0, 1)).Clamped().Hex()
}

func clampFloat(val, minVal, maxVal float64) float64 {
	return min(max(val, minVal), maxVal)
}

func relativeLuminance(c colorful.Color) float64 {
	r, g, b := c.LinearRgb()
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func kongLightPalette() Palette {
	return Palette{
		Name:        DefaultName,
		DisplayName: "Kong Light",
		About:       "Kong light theme based on the 2026 brand guidelines.",
		Colors: map[Token]Color{
			ColorTextPrimary:   singleColor("#000F06"),
			ColorTextSecondary: singleColor("#4A4D49"),
			ColorTextMuted:     singleColor("#676B66"),
			ColorBorder:        singleColor("#CDD4CB"),
			ColorSurface:       singleColor("#FFFFFF"),
			ColorSurfaceText:   singleColor("#000F06"),
			ColorPrimary:       singleColor("#000F06"),
			ColorPrimaryText:   singleColor("#FFFFFF"),
			ColorAccent:        singleColor("#CCFF00"),
			ColorAccentText:    singleColor("#000F06"),
			ColorSuccess:       singleColor("#000F06"),
			ColorSuccessText:   singleColor("#FFFFFF"),
			ColorInfo:          singleColor("#B7BDB5"),
			ColorInfoText:      singleColor("#000F06"),
			ColorWarning:       singleColor("#CCFF00"),
			ColorWarningText:   singleColor("#000F06"),
			ColorDanger:        singleColor("#2D2E2C"),
			ColorDangerText:    singleColor("#FFFFFF"),
			ColorHighlight:     singleColor("#E7EDE5"),
		},
	}
}

func kongDarkPalette() Palette {
	return Palette{
		Name:        "kong-dark",
		DisplayName: "Kong Dark",
		About:       "Kong dark theme based on the 2026 brand guidelines.",
		Colors: map[Token]Color{
			ColorTextPrimary:   singleColor("#FFFFFF"),
			ColorTextSecondary: singleColor("#D7DED4"),
			ColorTextMuted:     singleColor("#B7BDB5"),
			ColorBorder:        singleColor("#2D2E2C"),
			ColorSurface:       singleColor("#000F06"),
			ColorSurfaceText:   singleColor("#FFFFFF"),
			ColorPrimary:       singleColor("#CCFF00"),
			ColorPrimaryText:   singleColor("#000F06"),
			ColorAccent:        singleColor("#B7BDB5"),
			ColorAccentText:    singleColor("#000F06"),
			ColorSuccess:       singleColor("#CCFF00"),
			ColorSuccessText:   singleColor("#000F06"),
			ColorInfo:          singleColor("#B7BDB5"),
			ColorInfoText:      singleColor("#000F06"),
			ColorWarning:       singleColor("#CCFF00"),
			ColorWarningText:   singleColor("#000F06"),
			ColorDanger:        singleColor("#4A4D49"),
			ColorDangerText:    singleColor("#FFFFFF"),
			ColorHighlight:     singleColor("#2D2E2C"),
		},
	}
}
