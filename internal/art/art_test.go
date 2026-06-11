package art

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestRenderLoginBannerUses48ColumnAsset(t *testing.T) {
	var out strings.Builder

	if err := RenderLoginBanner(&out); err != nil {
		t.Fatalf("RenderLoginBanner: %v", err)
	}

	want, err := assets.ReadFile("assets/kong-48-braille.txt")
	if err != nil {
		t.Fatalf("read embedded login asset: %v", err)
	}
	if got := out.String(); got != string(want) {
		t.Fatalf("login banner used unexpected asset")
	}
}

func TestRenderKongBannerWritesSupportedWidths(t *testing.T) {
	for _, width := range SupportedKongBannerWidths() {
		t.Run(fmt.Sprintf("%d", width), func(t *testing.T) {
			var out strings.Builder
			if err := RenderKongBanner(&out, width); err != nil {
				t.Fatalf("RenderKongBanner: %v", err)
			}

			want, err := assets.ReadFile(fmt.Sprintf("assets/kong-%d-braille.txt", width))
			if err != nil {
				t.Fatalf("read embedded asset: %v", err)
			}
			if got := out.String(); got != string(want) {
				t.Fatalf("banner output did not match width %d asset", width)
			}
			if got := maxLineWidth(out.String()); got != width {
				t.Fatalf("banner width = %d, want %d", got, width)
			}
		})
	}
}

func TestRenderKongBannerTypeWritesSupportedTypes(t *testing.T) {
	for _, bannerType := range SupportedKongBannerTypes() {
		for _, width := range SupportedKongBannerWidths() {
			t.Run(fmt.Sprintf("%s-%d", bannerType, width), func(t *testing.T) {
				var out strings.Builder
				if err := RenderKongBannerType(&out, width, bannerType); err != nil {
					t.Fatalf("RenderKongBannerType: %v", err)
				}

				want, err := assets.ReadFile(fmt.Sprintf("assets/kong-%d-%s.txt", width, bannerType))
				if err != nil {
					t.Fatalf("read embedded asset: %v", err)
				}
				if got := out.String(); got != string(want) {
					t.Fatalf("banner output did not match %s width %d asset", bannerType, width)
				}
				if got := maxLineWidth(out.String()); got != width {
					t.Fatalf("banner width = %d, want %d", got, width)
				}
			})
		}
	}
}

func TestRenderKongBannerRejectsUnsupportedWidth(t *testing.T) {
	var out strings.Builder
	err := RenderKongBanner(&out, 72)
	if err == nil {
		t.Fatal("expected unsupported width error")
	}
	if !strings.Contains(err.Error(), "unsupported kong banner width 72") {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no output for unsupported width, got %q", out.String())
	}
}

func TestRenderKongBannerTypeRejectsUnsupportedType(t *testing.T) {
	var out strings.Builder
	err := RenderKongBannerType(&out, 48, KongBannerType("emoji"))
	if err == nil {
		t.Fatal("expected unsupported type error")
	}
	if !strings.Contains(err.Error(), `unsupported kong banner type "emoji"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no output for unsupported type, got %q", out.String())
	}
}

func TestSupportedKongBannerWidthsReturnsCopy(t *testing.T) {
	widths := SupportedKongBannerWidths()
	if len(widths) == 0 {
		t.Fatal("expected supported widths")
	}
	widths[0] = 999
	if got := SupportedKongBannerWidths()[0]; got == 999 {
		t.Fatal("SupportedKongBannerWidths returned mutable backing slice")
	}
}

func TestSupportedKongBannerTypesReturnsCopy(t *testing.T) {
	bannerTypes := SupportedKongBannerTypes()
	if len(bannerTypes) == 0 {
		t.Fatal("expected supported types")
	}
	bannerTypes[0] = "changed"
	if got := SupportedKongBannerTypes()[0]; got == "changed" {
		t.Fatal("SupportedKongBannerTypes returned mutable backing slice")
	}
}

func TestKongBannerAssetsUseBraillePatterns(t *testing.T) {
	for _, width := range SupportedKongBannerWidths() {
		name := fmt.Sprintf("assets/kong-%d-braille.txt", width)
		content, err := assets.ReadFile(name)
		if err != nil {
			t.Fatalf("read embedded asset %q: %v", name, err)
		}

		assertBrailleAsset(t, name, string(content))
	}
}

func TestKongBannerASCIIAssetsUseASCII(t *testing.T) {
	for _, width := range SupportedKongBannerWidths() {
		name := fmt.Sprintf("assets/kong-%d-ascii.txt", width)
		content, err := assets.ReadFile(name)
		if err != nil {
			t.Fatalf("read embedded asset %q: %v", name, err)
		}

		assertASCIIAsset(t, name, string(content))
	}
}

func TestRenderLoginBannerAssetsUseBraillePatterns(t *testing.T) {
	var out strings.Builder
	if err := RenderLoginBanner(&out); err != nil {
		t.Fatalf("RenderLoginBanner: %v", err)
	}
	assertBrailleAsset(t, "login banner", out.String())
}

func TestRenderLoginBannerNilWriter(t *testing.T) {
	if err := RenderLoginBanner(nil); err != nil {
		t.Fatalf("RenderLoginBanner(nil): %v", err)
	}
}

func TestRenderKongBannerNilWriter(t *testing.T) {
	if err := RenderKongBanner(nil, 48); err != nil {
		t.Fatalf("RenderKongBanner(nil): %v", err)
	}
}

func TestRenderKongBannerTypeNilWriter(t *testing.T) {
	if err := RenderKongBannerType(nil, 48, KongBannerASCII); err != nil {
		t.Fatalf("RenderKongBannerType(nil): %v", err)
	}
}

func assertBrailleAsset(t *testing.T, name, content string) {
	t.Helper()
	if strings.Contains(content, `\u`) {
		t.Fatalf("asset %q contains escaped unicode instead of rendered glyphs", name)
	}

	hasBraille := false
	for _, r := range content {
		switch {
		case r == '\n' || r == '\r' || r == ' ':
			continue
		case r >= '\u2800' && r <= '\u28ff':
			hasBraille = true
		default:
			t.Fatalf("asset %q contains non-braille rune %q", name, r)
		}
	}
	if !hasBraille {
		t.Fatalf("asset %q did not contain braille pattern glyphs", name)
	}
}

func assertASCIIAsset(t *testing.T, name, content string) {
	t.Helper()
	for _, r := range content {
		if r > '\u007f' {
			t.Fatalf("asset %q contains non-ASCII rune %q", name, r)
		}
	}
}

func maxLineWidth(value string) int {
	maxWidth := 0
	for line := range strings.Lines(value) {
		maxWidth = max(maxWidth, runewidth.StringWidth(strings.TrimSuffix(line, "\n")))
	}
	return maxWidth
}
