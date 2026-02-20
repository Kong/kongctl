package jq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/itchyny/gojq"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	FlagName                    = "jq"
	ColorFlagName               = "jq-color"
	ColorThemeFlagName          = "jq-color-theme"
	RawOutputFlagName           = "jq-raw-output"
	RawOutputFlagShort          = "r"
	DefaultExpressionConfigPath = "jq.default-expression"
	ColorEnabledConfigPath      = "jq.color.enabled"
	ColorThemeConfigPath        = "jq.color.theme"
	RawOutputConfigPath         = "jq.raw-output"
	DefaultTheme                = "friendly"
)

var jqQueryCache sync.Map

type Settings struct {
	Filter    string
	ColorMode cmdcommon.ColorMode
	Theme     string
	RawOutput bool
}

func AddFlags(flags *pflag.FlagSet) {
	flags.String(
		FlagName,
		"",
		"Filter JSON responses using jq expressions (powered by gojq for full jq compatibility)",
	)

	jqColor := cmdpkg.NewEnum([]string{
		cmdcommon.ColorModeAuto.String(),
		cmdcommon.ColorModeAlways.String(),
		cmdcommon.ColorModeNever.String(),
	}, cmdcommon.DefaultColorMode)

	flags.Var(
		jqColor,
		ColorFlagName,
		fmt.Sprintf(`Controls colorized output for jq filter results.
- Config path: [ %s ]
- Allowed    : [ auto|always|never ]`, ColorEnabledConfigPath),
	)

	flags.String(
		ColorThemeFlagName,
		DefaultTheme,
		fmt.Sprintf(`Select the color theme used for jq filter results.
- Config path: [ %s ]
- Examples   : [ friendly, github-dark, dracula ]
- Reference  : [ https://xyproto.github.io/splash/docs/ ]`, ColorThemeConfigPath),
	)

	flags.BoolP(
		RawOutputFlagName,
		RawOutputFlagShort,
		false,
		fmt.Sprintf(`Output string jq results without JSON quotes (like jq -r).
- Config path: [ %s ]`, RawOutputConfigPath),
	)
}

func bindFlag(cfg config.Hook, flags *pflag.FlagSet, flagName, configPath string) error {
	if f := flags.Lookup(flagName); f != nil {
		return cfg.BindFlag(configPath, f)
	}
	return nil
}

func BindFlags(cfg config.Hook, flags *pflag.FlagSet) error {
	if cfg == nil || flags == nil {
		return nil
	}

	bindings := []struct{ flag, cfgPath string }{
		{ColorFlagName, ColorEnabledConfigPath},
		{ColorThemeFlagName, ColorThemeConfigPath},
		{RawOutputFlagName, RawOutputConfigPath},
	}

	for _, b := range bindings {
		if err := bindFlag(cfg, flags, b.flag, b.cfgPath); err != nil {
			return err
		}
	}
	return nil
}

func applyDefaultExpression(settings *Settings, cfg config.Hook, flags *pflag.FlagSet) {
	if flags.Changed(FlagName) {
		return
	}
	defaultExpression := strings.TrimSpace(cfg.GetString(DefaultExpressionConfigPath))
	if defaultExpression != "" {
		settings.Filter = defaultExpression
	}
}

func applyColorSettings(settings *Settings, cfg config.Hook) error {
	colorValue := strings.ToLower(strings.TrimSpace(cfg.GetString(ColorEnabledConfigPath)))
	colorMode, err := cmdcommon.ColorModeStringToIota(colorValue)
	if err != nil {
		return err
	}
	settings.ColorMode = colorMode

	themeValue := strings.TrimSpace(cfg.GetString(ColorThemeConfigPath))
	if themeValue != "" {
		settings.Theme = themeValue
	}
	return nil
}

func applyFallbackSettings(settings *Settings, flags *pflag.FlagSet) error {
	if flags.Lookup(RawOutputFlagName) == nil {
		return nil
	}
	rawOutput, err := flags.GetBool(RawOutputFlagName)
	if err != nil {
		return err
	}
	settings.RawOutput = rawOutput
	return nil
}

func applyConfigSettings(settings *Settings, cfg config.Hook, flags *pflag.FlagSet) error {
	if cfg == nil {
		return applyFallbackSettings(settings, flags)
	}

	applyDefaultExpression(settings, cfg, flags)

	if err := applyColorSettings(settings, cfg); err != nil {
		return err
	}
	settings.RawOutput = cfg.GetBool(RawOutputConfigPath)
	return nil
}

func ResolveSettings(command *cobra.Command, cfg config.Hook) (Settings, error) {
	settings := Settings{
		Filter:    "",
		Theme:     DefaultTheme,
		ColorMode: cmdcommon.ColorModeAuto,
	}

	if command == nil {
		return settings, nil
	}

	flags := command.Flags()
	if flags == nil {
		return settings, nil
	}

	if flags.Lookup(FlagName) == nil {
		// Commands without --jq support should not implicitly enable jq via config.
		return settings, nil
	}

	jqFilter, err := flags.GetString(FlagName)
	if err != nil {
		return Settings{}, err
	}
	jqFilter = strings.TrimSpace(jqFilter)
	if flags.Changed(FlagName) && jqFilter == "" {
		jqFilter = "."
	}
	settings.Filter = jqFilter

	if err := applyConfigSettings(&settings, cfg, flags); err != nil {
		return Settings{}, err
	}

	return settings, nil
}

func HasFilter(settings Settings) bool {
	return strings.TrimSpace(settings.Filter) != ""
}

func ValidateOutputFormat(outType cmdcommon.OutputFormat, settings Settings) error {
	if settings.RawOutput {
		if !HasFilter(settings) {
			return &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("--%s requires --%s", RawOutputFlagName, FlagName),
			}
		}

		if outType != cmdcommon.JSON {
			return &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("--%s is only supported with --output json when used with --%s",
					RawOutputFlagName, FlagName),
			}
		}

		return nil
	}

	if !HasFilter(settings) {
		return nil
	}
	if outType == cmdcommon.JSON || outType == cmdcommon.YAML {
		return nil
	}
	return &cmdpkg.ConfigurationError{
		Err: fmt.Errorf("--%s is only supported with --output json or --output yaml", FlagName),
	}
}

func ApplyToRaw(raw any, outType cmdcommon.OutputFormat, settings Settings, out io.Writer) (any, bool, error) {
	if !HasFilter(settings) {
		return raw, false, nil
	}

	if err := ValidateOutputFormat(outType, settings); err != nil {
		return nil, false, err
	}

	body, err := json.Marshal(raw)
	if err != nil {
		return nil, false, fmt.Errorf("failed to encode output before applying jq filter: %w", err)
	}

	if settings.RawOutput {
		if err := ApplyRawFilter(body, settings.Filter, out); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}

	filtered, err := ApplyFilter(body, settings.Filter)
	if err != nil {
		return nil, false, err
	}

	if outType == cmdcommon.JSON && ShouldUseColor(settings.ColorMode, out) {
		formatted := BodyToPrintable(filtered)
		printable := MaybeColorizeOutput(filtered, formatted, settings.Theme)
		if _, err := fmt.Fprintln(out, strings.TrimRight(printable, "\n")); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}

	var payload any
	if len(filtered) > 0 {
		if err := json.Unmarshal(filtered, &payload); err != nil {
			payload = strings.TrimRight(BodyToPrintable(filtered), "\n")
		}
	}

	return payload, false, nil
}

func ApplyFilter(body []byte, filter string) ([]byte, error) {
	results, err := evaluateFilterResults(body, filter)
	if err != nil {
		return nil, err
	}

	return encodeFilterResults(results)
}

func ApplyRawFilter(body []byte, filter string, out io.Writer) error {
	results, err := evaluateFilterResults(body, filter)
	if err != nil {
		return err
	}

	return writeRawResults(results, out)
}

func evaluateFilterResults(body []byte, filter string) ([]any, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		filter = "."
	}

	if len(body) == 0 {
		return nil, errors.New("response body is empty, cannot apply jq filter")
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("response is not valid JSON: %w", err)
	}

	query, err := getCachedQuery(filter)
	if err != nil {
		return nil, err
	}

	iter := query.Run(payload)
	var results []any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			return nil, fmt.Errorf("jq filter failed: %w", err)
		}
		results = append(results, normalizeGoJQValue(v))
	}

	return results, nil
}

func encodeFilterResults(results []any) ([]byte, error) {
	if len(results) == 0 {
		return []byte("null"), nil
	}

	if len(results) == 1 {
		filtered, err := json.Marshal(results[0])
		if err != nil {
			return nil, fmt.Errorf("failed to encode filtered result: %w", err)
		}
		return filtered, nil
	}

	filtered, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("failed to encode filtered result: %w", err)
	}

	return filtered, nil
}

func writeRawResults(results []any, out io.Writer) error {
	for _, result := range results {
		if err := writeRawValue(result, out); err != nil {
			return err
		}
	}

	return nil
}

func writeRawValue(value any, out io.Writer) error {
	var line string
	if str, ok := value.(string); ok {
		line = str
	} else {
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to encode filtered result: %w", err)
		}
		line = string(encoded)
	}

	if _, err := fmt.Fprintln(out, line); err != nil {
		return err
	}

	return nil
}

func getCachedQuery(filter string) (*gojq.Code, error) {
	if code, ok := jqQueryCache.Load(filter); ok {
		cached, ok := code.(*gojq.Code)
		if !ok {
			return nil, fmt.Errorf("invalid cached jq code for filter %q", filter)
		}
		return cached, nil
	}

	parsed, err := gojq.Parse(filter)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression: %w", err)
	}

	code, err := gojq.Compile(parsed)
	if err != nil {
		return nil, fmt.Errorf("failed to compile jq expression: %w", err)
	}

	jqQueryCache.Store(filter, code)
	return code, nil
}

func normalizeGoJQValue(v any) any {
	switch value := v.(type) {
	case map[any]any:
		converted := make(map[string]any, len(value))
		for k, val := range value {
			converted[fmt.Sprint(k)] = normalizeGoJQValue(val)
		}
		return converted
	case []any:
		for i := range value {
			value[i] = normalizeGoJQValue(value[i])
		}
		return value
	default:
		return value
	}
}

func BodyToPrintable(body []byte) string {
	var js any
	if err := json.Unmarshal(body, &js); err != nil {
		return string(body)
	}
	formatted, err := json.MarshalIndent(js, "", "  ")
	if err != nil {
		return string(body)
	}
	return string(formatted)
}

var terminalDetector = func(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func ShouldUseColor(mode cmdcommon.ColorMode, out io.Writer) bool {
	switch mode {
	case cmdcommon.ColorModeAlways:
		return true
	case cmdcommon.ColorModeNever:
		return false
	case cmdcommon.ColorModeAuto:
		if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
			return false
		}
		return isTerminal(out)
	default:
		if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
			return false
		}
		return isTerminal(out)
	}
}

func isTerminal(out io.Writer) bool {
	type fdWriter interface {
		Fd() uintptr
	}
	if fw, ok := out.(fdWriter); ok {
		return terminalDetector(fw.Fd())
	}
	return false
}

func MaybeColorizeOutput(raw []byte, formatted, theme string) string {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return formatted
	}
	switch payload.(type) {
	case map[string]any, []any:
		// acceptable for colorization
	default:
		return formatted
	}

	lexer := lexers.Get("json")
	if lexer == nil {
		return formatted
	}
	iterator, err := lexer.Tokenise(nil, formatted)
	if err != nil {
		return formatted
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Get("terminal")
	}
	if formatter == nil {
		return formatted
	}

	style := styles.Get(theme)
	if style == nil {
		style = styles.Get(DefaultTheme)
	}
	if style == nil {
		style = styles.Fallback
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return formatted
	}

	return buf.String()
}
