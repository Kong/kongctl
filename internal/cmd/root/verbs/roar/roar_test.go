package roar

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/art"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
)

func TestResolveBannerWidthAutoSelectsLargestWidthThatFits(t *testing.T) {
	tests := []struct {
		name          string
		terminalWidth int
		want          int
	}{
		{
			name:          "below smallest",
			terminalWidth: 40,
			want:          48,
		},
		{
			name:          "fits 48",
			terminalWidth: 80,
			want:          48,
		},
		{
			name:          "fits 88",
			terminalWidth: 100,
			want:          88,
		},
		{
			name:          "fits 104",
			terminalWidth: 110,
			want:          104,
		},
		{
			name:          "fits 120",
			terminalWidth: 140,
			want:          120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubTerminalWidth(t, tt.terminalWidth, true)

			got, err := resolveBannerWidth(autoWidthValue, io.Discard)
			if err != nil {
				t.Fatalf("resolveBannerWidth: %v", err)
			}
			if got != tt.want {
				t.Fatalf("width = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestResolveBannerWidthAutoFallsBackWhenTerminalWidthUnavailable(t *testing.T) {
	stubTerminalWidth(t, 0, false)

	got, err := resolveBannerWidth(autoWidthValue, io.Discard)
	if err != nil {
		t.Fatalf("resolveBannerWidth: %v", err)
	}
	if got != fallbackWidth {
		t.Fatalf("width = %d, want %d", got, fallbackWidth)
	}
}

func TestResolveBannerWidthAcceptsExplicitSupportedWidth(t *testing.T) {
	got, err := resolveBannerWidth("88", io.Discard)
	if err != nil {
		t.Fatalf("resolveBannerWidth: %v", err)
	}
	if got != 88 {
		t.Fatalf("width = %d, want 88", got)
	}
}

func TestResolveBannerWidthRejectsUnsupportedWidth(t *testing.T) {
	_, err := resolveBannerWidth("72", io.Discard)
	if err == nil {
		t.Fatal("expected unsupported width error")
	}
	if !strings.Contains(err.Error(), "unsupported kong banner width 72") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveBannerWidthRejectsInvalidValue(t *testing.T) {
	_, err := resolveBannerWidth("wide", io.Discard)
	if err == nil {
		t.Fatal("expected invalid width error")
	}
	if !strings.Contains(err.Error(), `--width must be "auto"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveBannerTypeAutoDefaultsToBrailleForUTF8Terminals(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")

	got, err := resolveBannerType(autoArtValue)
	if err != nil {
		t.Fatalf("resolveBannerType: %v", err)
	}
	if got != art.KongBannerBraille {
		t.Fatalf("banner type = %s, want %s", got, art.KongBannerBraille)
	}
}

func TestResolveBannerTypeAutoUsesASCIIForDumbTerminal(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("LC_ALL", "en_US.UTF-8")

	got, err := resolveBannerType(autoArtValue)
	if err != nil {
		t.Fatalf("resolveBannerType: %v", err)
	}
	if got != art.KongBannerASCII {
		t.Fatalf("banner type = %s, want %s", got, art.KongBannerASCII)
	}
}

func TestResolveBannerTypeAutoUsesASCIIForNonUTF8Locale(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "C")

	got, err := resolveBannerType(autoArtValue)
	if err != nil {
		t.Fatalf("resolveBannerType: %v", err)
	}
	if got != art.KongBannerASCII {
		t.Fatalf("banner type = %s, want %s", got, art.KongBannerASCII)
	}
}

func TestResolveBannerTypeAcceptsExplicitSupportedTypes(t *testing.T) {
	tests := []struct {
		value string
		want  art.KongBannerType
	}{
		{
			value: "ascii",
			want:  art.KongBannerASCII,
		},
		{
			value: "braille",
			want:  art.KongBannerBraille,
		},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := resolveBannerType(tt.value)
			if err != nil {
				t.Fatalf("resolveBannerType: %v", err)
			}
			if got != tt.want {
				t.Fatalf("banner type = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestResolveBannerTypeRejectsInvalidValue(t *testing.T) {
	_, err := resolveBannerType("sixel")
	if err == nil {
		t.Fatal("expected invalid art error")
	}
	if !strings.Contains(err.Error(), `--art must be "auto"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderRoarBannerColorAlwaysAddsANSI(t *testing.T) {
	var out bytes.Buffer

	if err := renderRoarBanner(&out, 48, art.KongBannerBraille, cmdcommon.ColorModeAlways); err != nil {
		t.Fatalf("renderRoarBanner: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "\x1b[") {
		t.Fatalf("expected ANSI escape sequences in colored output:\n%q", output)
	}
	if !containsBraillePattern(output) {
		t.Fatalf("expected colored output to include braille banner:\n%s", output)
	}
}

func TestRenderRoarBannerColorNeverWritesPlainBanner(t *testing.T) {
	var out bytes.Buffer

	if err := renderRoarBanner(&out, 48, art.KongBannerBraille, cmdcommon.ColorModeNever); err != nil {
		t.Fatalf("renderRoarBanner: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("expected plain output without ANSI escape sequences:\n%q", output)
	}
	if !containsBraillePattern(output) {
		t.Fatalf("expected plain output to include braille banner:\n%s", output)
	}
}

func TestRenderRoarBannerWritesASCIIBanner(t *testing.T) {
	var out bytes.Buffer

	if err := renderRoarBanner(&out, 48, art.KongBannerASCII, cmdcommon.ColorModeNever); err != nil {
		t.Fatalf("renderRoarBanner: %v", err)
	}

	output := out.String()
	if containsBraillePattern(output) {
		t.Fatalf("expected ASCII output without braille glyphs:\n%s", output)
	}
	if !strings.Contains(output, "@") {
		t.Fatalf("expected ASCII output to include ASCII art:\n%s", output)
	}
}

func TestShouldColorizeBannerAutoRequiresTerminalAndHonorsNoColor(t *testing.T) {
	unsetEnvForTest(t, "NO_COLOR")
	stubTerminalWidth(t, 80, true)
	if !shouldColorizeBanner(cmdcommon.ColorModeAuto, io.Discard) {
		t.Fatal("expected auto color to be enabled for terminal output")
	}

	t.Setenv("NO_COLOR", "1")
	if shouldColorizeBanner(cmdcommon.ColorModeAuto, io.Discard) {
		t.Fatal("expected NO_COLOR to disable auto color")
	}
}

func TestShouldColorizeBannerAutoSkipsNonTerminal(t *testing.T) {
	unsetEnvForTest(t, "NO_COLOR")
	stubTerminalWidth(t, 0, false)
	if shouldColorizeBanner(cmdcommon.ColorModeAuto, io.Discard) {
		t.Fatal("expected auto color to be disabled when terminal width is unavailable")
	}
}

func TestShouldColorizeBannerAlwaysIgnoresNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if !shouldColorizeBanner(cmdcommon.ColorModeAlways, io.Discard) {
		t.Fatal("expected always color mode to force color")
	}
}

func stubTerminalWidth(t *testing.T, width int, ok bool) {
	t.Helper()
	original := detectTerminalWidth
	t.Cleanup(func() {
		detectTerminalWidth = original
	})
	detectTerminalWidth = func(io.Writer) (int, bool) {
		return width, ok
	}
}

func containsBraillePattern(s string) bool {
	for _, r := range s {
		if r >= '\u2800' && r <= '\u28ff' {
			return true
		}
	}
	return false
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	original, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if ok {
			if err := os.Setenv(key, original); err != nil {
				t.Fatalf("restore %s: %v", key, err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("restore unset %s: %v", key, err)
		}
	})
}
