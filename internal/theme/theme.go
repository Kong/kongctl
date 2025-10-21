package theme

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	tint "github.com/lrstanley/bubbletint"
	"github.com/lucasb-eyer/go-colorful"
)

// DefaultName is the built-in theme used when no override is provided.
const DefaultName = "kong"

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

// Adaptive converts the color into a lipgloss adaptive color.
func (c Color) Adaptive() lipgloss.AdaptiveColor {
	light, dark := strings.TrimSpace(c.Light), strings.TrimSpace(c.Dark)
	switch {
	case light == "" && dark == "":
		return lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}
	case light == "":
		light = dark
	case dark == "":
		dark = light
	}
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
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

// Adaptive returns the lipgloss adaptive color for the provided token.
func (p Palette) Adaptive(token Token) lipgloss.AdaptiveColor {
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
	registryOnce sync.Once
	registryMu   sync.RWMutex
	palettes     map[string]Palette
	current      Palette
	defaultPal   Palette
	themeKey     contextKey
)

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

	keys := make([]string, 0, len(palettes))
	for k := range palettes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
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

	_, ok := palettes[sanitizeName(name)]
	return ok
}

// Get returns the palette with the provided name.
func Get(name string) (Palette, bool) {
	ensureRegistry()

	registryMu.RLock()
	defer registryMu.RUnlock()

	p, ok := palettes[sanitizeName(name)]
	return p, ok
}

// SetCurrent sets the active palette.
func SetCurrent(name string) error {
	ensureRegistry()

	name = sanitizeName(name)
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

// Current returns the active palette.
func Current() Palette {
	ensureRegistry()

	registryMu.RLock()
	defer registryMu.RUnlock()

	return current
}

// CurrentName returns the ID of the active palette.
func CurrentName() string {
	return sanitizeName(Current().Name)
}

// Flag is a pflag.Value implementation for theme IDs.
type Flag struct {
	value string
}

// NewFlag returns a Flag with the provided default value.
func NewFlag(defaultValue string) *Flag {
	name := sanitizeName(defaultValue)
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
	name := sanitizeName(v)
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

		registerPalette(kongPalette())
		defaultPal = palettes[DefaultName]
		current = defaultPal

		for _, t := range tint.DefaultTints() {
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

func paletteFromTint(t tint.Tint) Palette {
	if t == nil {
		return Palette{}
	}

	fg := normalizeHex(tint.Hex(t.Fg()))
	bg := normalizeHex(tint.Hex(t.Bg()))
	muted := normalizeHex(tint.Hex(t.BrightBlack()))
	accent := normalizeHex(tint.Hex(t.Cyan()))
	accentBright := normalizeHex(tint.Hex(t.BrightBlue()))
	success := normalizeHex(tint.Hex(t.Green()))
	info := normalizeHex(tint.Hex(t.Blue()))
	warning := normalizeHex(tint.Hex(t.Yellow()))
	danger := normalizeHex(tint.Hex(t.Red()))
	highlight := normalizeHex(tint.Hex(t.BrightWhite()))

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
		Name:        sanitizeName(t.ID()),
		DisplayName: strings.TrimSpace(t.DisplayName()),
		About:       strings.TrimSpace(t.About()),
		Colors:      colors,
	}
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
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

func relativeLuminance(c colorful.Color) float64 {
	r, g, b := c.LinearRgb()
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func kongPalette() Palette {
	return Palette{
		Name:        DefaultName,
		DisplayName: "Kong (Default)",
		About:       "Kong-branded palette inspired by the Kai experience.",
		Colors: map[Token]Color{
			ColorTextPrimary:   pairColor("#0A0A0A", "#FFFFFF"),
			ColorTextSecondary: pairColor("#1F2026", "#D7D9E3"),
			ColorTextMuted:     pairColor("#8B8FA3", "#6A6F85"),
			ColorBorder:        pairColor("#4A4D65", "#4A4D65"),
			ColorSurface:       pairColor("#FFFFFF", "#121418"),
			ColorSurfaceText:   pairColor("#0A0A0A", "#FFFFFF"),
			ColorPrimary:       pairColor("#0C7C51", "#0C7C51"),
			ColorPrimaryText:   pairColor("#121418", "#121418"),
			ColorAccent:        pairColor("#F8C77E", "#F8C77E"),
			ColorAccentText:    pairColor("#121418", "#121418"),
			ColorSuccess:       pairColor("#0C7C51", "#0C7C51"),
			ColorSuccessText:   pairColor("#121418", "#121418"),
			ColorInfo:          pairColor("#286FEB", "#286FEB"),
			ColorInfoText:      pairColor("#0A0A0A", "#F8F8F8"),
			ColorWarning:       pairColor("#F8C77E", "#F8C77E"),
			ColorWarningText:   pairColor("#121418", "#121418"),
			ColorDanger:        pairColor("#E25D5D", "#E25D5D"),
			ColorDangerText:    pairColor("#121418", "#121418"),
			ColorHighlight:     pairColor("#E8EDF4", "#E8EDF4"),
		},
	}
}
