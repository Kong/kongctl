package sdk

import (
	"fmt"
	"io"
	"os"
	"strings"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/segmentio/cli"
)

// Output renders extension output using the parent kongctl output settings.
type Output struct {
	settings OutputContext
	writer   io.Writer
}

// Output returns a renderer configured from the runtime context.
func (r *RuntimeContext) Output() Output {
	settings := r.OutputSettings
	if strings.TrimSpace(settings.Format) == "" {
		settings.Format = r.Resolved.Output
	}
	return Output{
		settings: settings,
		writer:   os.Stdout,
	}
}

// WithWriter returns a copy of the renderer that writes to w.
func (o Output) WithWriter(w io.Writer) Output {
	o.writer = w
	return o
}

// Render writes display in text mode and raw in structured modes. When raw is
// omitted, display is used for both text and structured output.
func (o Output) Render(display any, raw ...any) error {
	out := o.writer
	if out == nil {
		out = os.Stdout
	}

	rawValue := display
	if len(raw) > 0 {
		rawValue = raw[0]
	}

	outType, err := cmdcommon.OutputFormatStringToIota(o.format())
	if err != nil {
		return err
	}

	jqSettings, err := o.jqSettings()
	if err != nil {
		return err
	}
	if err := jqoutput.ValidateOutputFormat(outType, jqSettings); err != nil {
		return err
	}
	if jqoutput.HasFilter(jqSettings) {
		filteredRaw, handled, err := jqoutput.ApplyToRaw(rawValue, outType, jqSettings, out)
		if err != nil {
			return fmt.Errorf("jq filter failed: %w", err)
		}
		if handled {
			return nil
		}
		rawValue = filteredRaw
	}

	printer, err := cli.Format(outType.String(), out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	switch outType {
	case cmdcommon.TEXT:
		printer.Print(display)
	case cmdcommon.JSON, cmdcommon.YAML:
		printer.Print(rawValue)
	default:
		return fmt.Errorf("unsupported output format %s", outType.String())
	}
	return nil
}

func (o Output) format() string {
	format := strings.TrimSpace(o.settings.Format)
	if format == "" {
		return cmdcommon.DefaultOutputFormat
	}
	return format
}

func (o Output) jqSettings() (jqoutput.Settings, error) {
	color := strings.TrimSpace(o.settings.JQ.Color)
	if color == "" {
		color = cmdcommon.DefaultColorMode
	}
	colorMode, err := cmdcommon.ColorModeStringToIota(color)
	if err != nil {
		return jqoutput.Settings{}, err
	}

	theme := strings.TrimSpace(o.settings.JQ.ColorTheme)
	if theme == "" {
		theme = jqoutput.DefaultTheme
	}

	return jqoutput.Settings{
		Filter:    strings.TrimSpace(o.settings.JQ.Expression),
		ColorMode: colorMode,
		Theme:     theme,
		RawOutput: o.settings.JQ.RawOutput,
	}, nil
}
