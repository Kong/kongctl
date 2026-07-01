package roar

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kong/kongctl/internal/art"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/mattn/go-runewidth"
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
			stubTerminalData(t, terminalCapabilities{width: tt.terminalWidth})

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
	stubTerminalData(t, terminalCapabilities{})

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
	if !strings.Contains(err.Error(), `--climber-width must be "auto"`) {
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
	if !strings.Contains(err.Error(), `--climber-art must be "auto"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClimberArtFlagUsageClarifiesClimberScope(t *testing.T) {
	cmd := NewRoarCmd()
	flag := cmd.Flags().Lookup(climberArtFlagName)
	if flag == nil {
		t.Fatal("expected climber-art flag")
		return
	}
	for _, want := range []string{
		"Climber banner art type",
		"selecting a concrete art type skips animation",
	} {
		if !strings.Contains(flag.Usage, want) {
			t.Fatalf("expected climber-art flag usage to contain %q, got %q", want, flag.Usage)
		}
	}

	legacyFlag := cmd.Flags().Lookup(legacyArtFlagName)
	if legacyFlag == nil {
		t.Fatal("expected legacy art flag")
		return
	}
	if !legacyFlag.Hidden {
		t.Fatal("expected legacy art flag to be hidden")
	}
}

func TestClimberFlagUsage(t *testing.T) {
	cmd := NewRoarCmd()
	flag := cmd.Flags().Lookup(climberFlagName)
	if flag == nil {
		t.Fatal("expected climber flag")
		return
	}
	if !strings.Contains(flag.Usage, "static climber banner") {
		t.Fatalf("expected climber flag usage to describe climber banner, got %q", flag.Usage)
	}
}

func TestClimberWidthFlagUsage(t *testing.T) {
	cmd := NewRoarCmd()
	flag := cmd.Flags().Lookup(climberWidthFlagName)
	if flag == nil {
		t.Fatal("expected climber-width flag")
		return
	}
	if !strings.Contains(flag.Usage, "Climber banner width") {
		t.Fatalf("expected climber-width flag usage to describe climber banner width, got %q", flag.Usage)
	}

	legacyFlag := cmd.Flags().Lookup(legacyWidthFlagName)
	if legacyFlag == nil {
		t.Fatal("expected legacy width flag")
		return
	}
	if !legacyFlag.Hidden {
		t.Fatal("expected legacy width flag to be hidden")
	}
}

func TestColorFlagDefaultsToNative(t *testing.T) {
	cmd := NewRoarCmd()
	flag := cmd.Flags().Lookup(colorFlagName)
	if flag == nil {
		t.Fatal("expected color flag")
		return
	}
	if flag.DefValue != nativeColorValue {
		t.Fatalf("default color = %q, want %q", flag.DefValue, nativeColorValue)
	}
	if !strings.Contains(flag.Usage, `"native"`) {
		t.Fatalf("expected color flag usage to mention native, got %q", flag.Usage)
	}
}

func TestClimberBannerFlagChanged(t *testing.T) {
	tests := []struct {
		name  string
		flag  string
		value string
		want  bool
	}{
		{
			name:  "climber art concrete type",
			flag:  climberArtFlagName,
			value: art.KongBannerBraille.String(),
			want:  true,
		},
		{
			name:  "climber art auto",
			flag:  climberArtFlagName,
			value: autoArtValue,
		},
		{
			name:  "legacy art concrete type",
			flag:  legacyArtFlagName,
			value: art.KongBannerBraille.String(),
			want:  true,
		},
		{
			name: "climber width",
			flag: climberWidthFlagName,
			want: true,
		},
		{
			name: "legacy width",
			flag: legacyWidthFlagName,
			want: true,
		},
		{
			name: "color",
			flag: colorFlagName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRoarCmd()
			if tt.flag != "" {
				value := tt.value
				if value == "" {
					value = cmd.Flags().Lookup(tt.flag).DefValue
				}
				if err := cmd.Flags().Set(tt.flag, value); err != nil {
					t.Fatalf("set %s: %v", tt.flag, err)
				}
			}

			got := climberBannerFlagChanged(cmd)
			if got != tt.want {
				t.Fatalf("climberBannerFlagChanged = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestResolveEffectiveBannerColorAutoSkipsNonTerminalOutput(t *testing.T) {
	got, err := resolveEffectiveBannerColor(autoColorValue, terminalCapabilities{isTTY: false})
	if err != nil {
		t.Fatalf("resolveEffectiveBannerColor: %v", err)
	}
	if got != nil {
		t.Fatalf("expected auto color to be disabled for non-terminal output, got %#v", got)
	}
}

func TestResolveEffectiveBannerColorAutoUsesTerminalOutput(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	unsetEnv(t, "NO_COLOR")

	got, err := resolveEffectiveBannerColor(autoColorValue, terminalCapabilities{isTTY: true})
	if err != nil {
		t.Fatalf("resolveEffectiveBannerColor: %v", err)
	}
	if got == nil {
		t.Fatal("expected auto color for terminal output")
	}
}

func TestResolveEffectiveBannerColorAutoHonorsNoColor(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "1")

	got, err := resolveEffectiveBannerColor(autoColorValue, terminalCapabilities{isTTY: true})
	if err != nil {
		t.Fatalf("resolveEffectiveBannerColor: %v", err)
	}
	if got != nil {
		t.Fatalf("expected NO_COLOR to disable auto color, got %#v", got)
	}
}

func TestShouldUseNativeAnimationColorRequiresColorTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	unsetEnv(t, "NO_COLOR")

	if shouldUseNativeAnimationColor(nativeColorValue, terminalCapabilities{isTTY: false}) {
		t.Fatal("expected non-terminal output not to use native animation color")
	}
	if !shouldUseNativeAnimationColor(nativeColorValue, terminalCapabilities{isTTY: true}) {
		t.Fatal("expected terminal output to use native animation color")
	}

	t.Setenv("NO_COLOR", "1")
	if shouldUseNativeAnimationColor(nativeColorValue, terminalCapabilities{isTTY: true}) {
		t.Fatal("expected NO_COLOR to disable native animation color")
	}
}

func TestResolveEffectiveBannerColorExplicitCodeBypassesTerminalDetection(t *testing.T) {
	got, err := resolveEffectiveBannerColor("#CCFF00", terminalCapabilities{isTTY: false})
	if err != nil {
		t.Fatalf("resolveEffectiveBannerColor: %v", err)
	}
	if got == nil {
		t.Fatal("expected explicit color code to resolve")
	}
}

func TestResolveBannerColorOffAndNativeDisableTint(t *testing.T) {
	for _, value := range []string{"", "native", "NATIVE", "off", "OFF"} {
		t.Run(value, func(t *testing.T) {
			got, err := resolveBannerColor(value)
			if err != nil {
				t.Fatalf("resolveBannerColor: %v", err)
			}
			if got != nil {
				t.Fatalf("expected %q to disable color, got %#v", value, got)
			}
		})
	}
}

func TestResolveBannerColorAcceptsAutoAndExplicitCodes(t *testing.T) {
	for _, value := range []string{"auto", "AUTO", "#CCFF00", "#cf0", "42"} {
		t.Run(value, func(t *testing.T) {
			got, err := resolveBannerColor(value)
			if err != nil {
				t.Fatalf("resolveBannerColor: %v", err)
			}
			if got == nil {
				t.Fatalf("expected %q to resolve to a color", value)
			}
		})
	}
}

func TestResolveBannerColorRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"sparkle", "always", "never", "#xyz", "256", "-1"} {
		t.Run(value, func(t *testing.T) {
			_, err := resolveBannerColor(value)
			if err == nil {
				t.Fatal("expected invalid color error")
			}
			if !strings.Contains(err.Error(), `--color must be "native", "auto", "off"`) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveRoarPlacementAcceptsSupportedValuesAndAliases(t *testing.T) {
	tests := []struct {
		value string
		want  roarPlacement
	}{
		{
			value: "",
			want:  placementTopLeft,
		},
		{
			value: "center",
			want:  placementCenter,
		},
		{
			value: "middle",
			want:  placementCenter,
		},
		{
			value: "top-left",
			want:  placementTopLeft,
		},
		{
			value: "top-center",
			want:  placementTop,
		},
		{
			value: "top-right",
			want:  placementTopRight,
		},
		{
			value: "center-left",
			want:  placementLeft,
		},
		{
			value: "right",
			want:  placementRight,
		},
		{
			value: "bottom-left",
			want:  placementBottomLeft,
		},
		{
			value: "bottom",
			want:  placementBottom,
		},
		{
			value: "bottom-right",
			want:  placementBottomRight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := resolveRoarPlacement(tt.value)
			if err != nil {
				t.Fatalf("resolveRoarPlacement: %v", err)
			}
			if got != tt.want {
				t.Fatalf("placement = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLocationFlagDefaultsToTopLeft(t *testing.T) {
	cmd := NewRoarCmd()
	flag := cmd.Flags().Lookup(locationFlagName)
	if flag == nil {
		t.Fatal("expected location flag")
		return
	}
	if flag.DefValue != string(placementTopLeft) {
		t.Fatalf("default location = %q, want %q", flag.DefValue, placementTopLeft)
	}

	legacyFlag := cmd.Flags().Lookup(legacyPlacementFlagName)
	if legacyFlag == nil {
		t.Fatal("expected legacy placement flag")
		return
	}
	if !legacyFlag.Hidden {
		t.Fatal("expected legacy placement flag to be hidden")
	}
}

func TestResolveRoarPlacementRejectsInvalidValue(t *testing.T) {
	_, err := resolveRoarPlacement("diagonal")
	if err == nil {
		t.Fatal("expected invalid placement error")
	}
	if !strings.Contains(err.Error(), "--location must be one of") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShouldRenderAnimationRequiresTerminalWithEnoughSpace(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	unsetEnv(t, "NO_COLOR")

	tests := []struct {
		name      string
		noAnimate bool
		terminal  terminalCapabilities
		want      bool
	}{
		{
			name: "supported",
			terminal: terminalCapabilities{
				width:  art.KongRoarAnimationWidth,
				height: art.KongRoarAnimationHeight,
				isTTY:  true,
			},
			want: true,
		},
		{
			name:      "disabled by flag",
			noAnimate: true,
			terminal:  terminalCapabilities{width: 120, height: 40, isTTY: true},
		},
		{
			name:      "not tty",
			terminal:  terminalCapabilities{width: 120, height: 40},
			noAnimate: false,
		},
		{
			name:     "too narrow",
			terminal: terminalCapabilities{width: art.KongRoarAnimationWidth - 1, height: 40, isTTY: true},
		},
		{
			name:     "too short",
			terminal: terminalCapabilities{width: 120, height: art.KongRoarAnimationHeight - 1, isTTY: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRenderAnimation(tt.noAnimate, tt.terminal)
			if got != tt.want {
				t.Fatalf("shouldRenderAnimation = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestShouldRenderAnimationSkipsDumbTerminal(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("LC_ALL", "en_US.UTF-8")

	got := shouldRenderAnimation(false, terminalCapabilities{width: 120, height: 40, isTTY: true})
	if got {
		t.Fatal("expected TERM=dumb to disable animation")
	}
}

func TestShouldRenderAnimationSkipsNonUTF8Locale(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "C")

	got := shouldRenderAnimation(false, terminalCapabilities{width: 120, height: 40, isTTY: true})
	if got {
		t.Fatal("expected non-UTF-8 locale to disable animation")
	}
}

func TestShouldRenderAnimationAllowsNoColor(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	t.Setenv("NO_COLOR", "1")

	got := shouldRenderAnimation(false, terminalCapabilities{width: 120, height: 40, isTTY: true})
	if !got {
		t.Fatal("expected color-disabled output to still allow animation")
	}
}

func TestRoarRunFallsBackToClimberInNarrowTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	streams, _, out, errOut := iostreams.NewTestIOStreams()
	stubTerminalData(t, terminalCapabilities{width: 60, height: 40, isTTY: true})

	if err := runRoarForTest(t, streams); err != nil {
		t.Fatalf("roar run: %v", err)
	}

	output := out.String()
	if !containsBraillePattern(output) {
		t.Fatalf("expected narrow terminal to use climber braille fallback:\n%s", output)
	}
	if got := maxLineWidth(output); got != fallbackWidth {
		t.Fatalf("fallback width = %d, want %d\noutput:\n%s", got, fallbackWidth, output)
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr output, got:\n%s", errOut.String())
	}
}

func TestRoarRunReportsTooNarrowTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	streams, _, out, errOut := iostreams.NewTestIOStreams()
	stubTerminalData(t, terminalCapabilities{width: fallbackWidth - 1, height: 40, isTTY: true})

	if err := runRoarForTest(t, streams); err != nil {
		t.Fatalf("roar run: %v", err)
	}

	if out.Len() != 0 {
		t.Fatalf("expected no stdout for too narrow terminal, got:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "kongctl roar requires a terminal at least 48 columns wide") {
		t.Fatalf("expected narrow terminal message, got:\n%s", errOut.String())
	}
}

func TestRenderRoarBannerAutoColorAddsANSI(t *testing.T) {
	var out bytes.Buffer
	bannerColor, err := resolveBannerColor("auto")
	if err != nil {
		t.Fatalf("resolveBannerColor: %v", err)
	}

	if err := renderRoarBanner(&out, 48, art.KongBannerBraille, bannerColor); err != nil {
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

func TestRenderRoarBannerExplicitColorAddsANSI(t *testing.T) {
	var out bytes.Buffer
	bannerColor, err := resolveBannerColor("#CCFF00")
	if err != nil {
		t.Fatalf("resolveBannerColor: %v", err)
	}

	if err := renderRoarBanner(&out, 48, art.KongBannerBraille, bannerColor); err != nil {
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

func TestRenderRoarBannerColorOffWritesPlainBanner(t *testing.T) {
	var out bytes.Buffer

	if err := renderRoarBanner(&out, 48, art.KongBannerBraille, nil); err != nil {
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

func TestColorizeAnimationFramesStripsNativeANSI(t *testing.T) {
	frames := prepareAnimationFrames(
		[]string{"\x1b[38;2;0;0;0;48;2;0;0;0m#\x1b[38;2;255;255;255;48;2;0;0;0m#\x1b[0m\n"},
		nil,
		false,
	)
	if len(frames) != 1 {
		t.Fatalf("frame count = %d, want 1", len(frames))
	}
	if strings.Contains(frames[0], "\x1b[") {
		t.Fatalf("expected animation frame ANSI to be stripped:\n%q", frames[0])
	}
	if frames[0] != " @\n" {
		t.Fatalf("plain frame = %q, want %q", frames[0], " @\n")
	}
}

func TestColorizeAnimationFramesAppliesWholeFrameColor(t *testing.T) {
	bannerColor, err := resolveBannerColor("#CCFF00")
	if err != nil {
		t.Fatalf("resolveBannerColor: %v", err)
	}

	frames := prepareAnimationFrames(
		[]string{"\x1b[38;2;0;0;0;48;2;0;0;0m#\x1b[38;2;255;255;255;48;2;0;0;0m#\x1b[0m\n"},
		bannerColor,
		false,
	)
	if len(frames) != 1 {
		t.Fatalf("frame count = %d, want 1", len(frames))
	}
	if !strings.Contains(frames[0], "\x1b[") {
		t.Fatalf("expected colorized animation frame to include ANSI:\n%q", frames[0])
	}
	if strings.Contains(frames[0], "\x1b[31m") {
		t.Fatalf("expected native frame color to be stripped before tinting:\n%q", frames[0])
	}
	if !strings.Contains(frames[0], " @") {
		t.Fatalf("expected colorized frame content to be preserved:\n%q", frames[0])
	}
}

func TestColorizeAnimationFramesPreservesEmbeddedFrameShape(t *testing.T) {
	sourceFrames, err := art.KongRoarAnimationFrames()
	if err != nil {
		t.Fatalf("KongRoarAnimationFrames: %v", err)
	}

	frames := prepareAnimationFrames(sourceFrames, nil, false)
	for _, frame := range frames {
		if strings.Contains(frame, "\x1b[") {
			t.Fatalf("expected converted animation frame without ANSI:\n%q", frame)
		}
		if strings.Contains(frame, " ") && strings.TrimSpace(frame) != "" {
			return
		}
	}
	t.Fatal("expected at least one converted animation frame to include spaces and visible glyphs")
}

func TestPrepareAnimationFramesPreservesNativeFrames(t *testing.T) {
	frames := prepareAnimationFrames([]string{"\x1b[38;2;255;255;255m#\x1b[0m\n"}, nil, true)
	if len(frames) != 1 {
		t.Fatalf("frame count = %d, want 1", len(frames))
	}
	if frames[0] != "\x1b[38;2;255;255;255m#\x1b[0m\n" {
		t.Fatalf("native frame = %q", frames[0])
	}
}

func TestSelectDefaultStaticFrameUsesFrameOneThirdThroughAnimation(t *testing.T) {
	if defaultStaticFrameNumber != 29 {
		t.Fatalf("default static frame number = %d, want 29", defaultStaticFrameNumber)
	}

	frames := make([]string, art.KongRoarAnimationFrameCount)
	for i := range frames {
		frames[i] = "other"
	}
	frames[defaultStaticFrameIndex] = "chosen"

	got, err := selectDefaultStaticFrame(frames)
	if err != nil {
		t.Fatalf("selectDefaultStaticFrame: %v", err)
	}
	if got != "chosen" {
		t.Fatalf("selected frame = %q, want chosen", got)
	}
}

func TestRenderRoarStaticFrameWritesConvertedAnimationFrame(t *testing.T) {
	var out bytes.Buffer

	if err := renderRoarStaticFrame(&out, nil, false); err != nil {
		t.Fatalf("renderRoarStaticFrame: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("expected static frame without ANSI escape sequences:\n%q", output)
	}
	if containsBraillePattern(output) {
		t.Fatalf("expected static frame not to include braille glyphs:\n%s", output)
	}
	if strings.TrimSpace(output) == "" {
		t.Fatal("expected static frame to include visible glyphs")
	}
	if got := lineCount(output); got != art.KongRoarAnimationHeight {
		t.Fatalf("static frame height = %d, want %d", got, art.KongRoarAnimationHeight)
	}
}

func TestRenderRoarStaticFramePreservesNativeAnimationColor(t *testing.T) {
	var out bytes.Buffer

	if err := renderRoarStaticFrame(&out, nil, true); err != nil {
		t.Fatalf("renderRoarStaticFrame: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "\x1b[") {
		t.Fatalf("expected native static frame to include ANSI escape sequences:\n%q", output)
	}
	if !strings.Contains(output, "#") {
		t.Fatalf("expected native static frame to include source cells:\n%q", output)
	}
}

func TestRoarAnimationModelUsesAltScreenAndCentersFrame(t *testing.T) {
	model := newRoarAnimationModel(
		[]string{"frame"},
		1,
		terminalCapabilities{width: 100, height: 40},
		placementCenter,
	)

	view := model.View()
	if !view.AltScreen {
		t.Fatal("expected animation view to use alt screen")
	}
	if !strings.Contains(view.Content, "frame") {
		t.Fatalf("expected view content to include current frame:\n%q", view.Content)
	}
	if got := lineCount(view.Content); got != 40 {
		t.Fatalf("centered view height = %d, want 40", got)
	}
}

func TestRoarAnimationModelPlacesFrame(t *testing.T) {
	tests := []struct {
		name      string
		placement roarPlacement
		assert    func(*testing.T, string)
	}{
		{
			name:      "top left",
			placement: placementTopLeft,
			assert: func(t *testing.T, content string) {
				t.Helper()
				if !strings.HasPrefix(content, "frame") {
					t.Fatalf("expected top-left placement to start with frame:\n%q", content)
				}
			},
		},
		{
			name:      "bottom right",
			placement: placementBottomRight,
			assert: func(t *testing.T, content string) {
				t.Helper()
				lines := strings.Split(content, "\n")
				lastLine := lines[len(lines)-1]
				if strings.TrimSpace(lastLine) != "frame" {
					t.Fatalf("expected bottom-right placement to end with frame:\n%q", content)
				}
				if !strings.HasPrefix(lastLine, " ") {
					t.Fatalf("expected bottom-right placement to right-align frame:\n%q", lastLine)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := newRoarAnimationModel(
				[]string{"frame"},
				1,
				terminalCapabilities{width: 100, height: 40},
				tt.placement,
			)

			tt.assert(t, model.View().Content)
		})
	}
}

func TestRoarAnimationModelAdvancesFramesAndQuits(t *testing.T) {
	model := newRoarAnimationModel([]string{"first", "second"}, 1, terminalCapabilities{}, placementCenter)

	nextModel, cmd := model.Update(roarAnimationTickMsg(time.Now()))
	advanced, ok := nextModel.(roarAnimationModel)
	if !ok {
		t.Fatalf("updated model type = %T, want roarAnimationModel", nextModel)
	}
	if advanced.frame != 1 {
		t.Fatalf("frame = %d, want 1", advanced.frame)
	}
	if cmd == nil {
		t.Fatal("expected next tick command")
	}
	if !strings.Contains(advanced.View().Content, "second") {
		t.Fatalf("expected second frame after tick:\n%q", advanced.View().Content)
	}

	_, cmd = advanced.Update(roarAnimationTickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("quit command message type = %T, want tea.QuitMsg", msg)
	}
}

func TestRoarAnimationModelQuitsOnCtrlC(t *testing.T) {
	model := newRoarAnimationModel([]string{"frame"}, 1, terminalCapabilities{}, placementCenter)

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("expected ctrl-c to return quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("quit command message type = %T, want tea.QuitMsg", msg)
	}
}

func TestRoarAnimationModelTracksWindowSize(t *testing.T) {
	model := newRoarAnimationModel(
		[]string{"frame"},
		1,
		terminalCapabilities{width: 80, height: 22},
		placementCenter,
	)

	nextModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	resized, ok := nextModel.(roarAnimationModel)
	if !ok {
		t.Fatalf("updated model type = %T, want roarAnimationModel", nextModel)
	}
	if resized.width != 100 || resized.height != 40 {
		t.Fatalf("window size = %dx%d, want 100x40", resized.width, resized.height)
	}
}

func TestRenderRoarAnimationRejectsInvalidLoopCount(t *testing.T) {
	var out bytes.Buffer
	err := renderRoarAnimation(context.Background(), nil, &out, 0, terminalCapabilities{}, placementCenter, nil, false)
	if err == nil {
		t.Fatal("expected invalid loop count error")
	}
}

func TestRenderRoarBannerWritesASCIIBanner(t *testing.T) {
	var out bytes.Buffer

	if err := renderRoarBanner(&out, 48, art.KongBannerASCII, nil); err != nil {
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

func stubTerminalData(t *testing.T, terminal terminalCapabilities) {
	t.Helper()
	original := detectTerminalData
	t.Cleanup(func() {
		detectTerminalData = original
	})
	detectTerminalData = func(io.Writer) terminalCapabilities {
		return terminal
	}
}

func runRoarForTest(t *testing.T, streams *iostreams.IOStreams, args ...string) error {
	t.Helper()
	cmd := NewRoarCmd()
	cmd.SetContext(context.WithValue(context.Background(), iostreams.StreamsKey, streams))
	cmd.SetArgs(args)
	return cmd.Execute()
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	original, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, original)
			return
		}
		_ = os.Unsetenv(key)
	})
}

func lineCount(value string) int {
	count := 0
	for range strings.Lines(value) {
		count++
	}
	return count
}

func maxLineWidth(value string) int {
	maxWidth := 0
	for line := range strings.Lines(value) {
		maxWidth = max(maxWidth, runewidth.StringWidth(strings.TrimSuffix(line, "\n")))
	}
	return maxWidth
}

func containsBraillePattern(s string) bool {
	for _, r := range s {
		if r >= '\u2800' && r <= '\u28ff' {
			return true
		}
	}
	return false
}
