package text

import (
	"fmt"
	"strings"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	LayoutFlagName     = "text-layout"
	LayoutConfigPath   = "text.layout"
	IDFormatFlagName   = "text-id-format"
	IDFormatConfigPath = "text.id-format"
	DefaultLayout      = LayoutCompact
	DefaultIDFormat    = IDFormatCompact
)

type Layout string

const (
	LayoutCompact Layout = "compact"
	LayoutAuto    Layout = "auto"
	LayoutWide    Layout = "wide"
)

type IDFormat string

const (
	IDFormatCompact IDFormat = "compact"
	IDFormatFull    IDFormat = "full"
)

type Settings struct {
	Layout   Layout
	IDFormat IDFormat
}

func AddFlags(flags *pflag.FlagSet) {
	if flags == nil {
		return
	}
	if flags.Lookup(LayoutFlagName) == nil {
		flags.String(LayoutFlagName, "", fmt.Sprintf(`Configure static text-table column selection.
- Config path: [ %s ]
- Allowed    : [ compact|auto|wide ]
- Default    : [ compact ]`, LayoutConfigPath))
	}
	if flags.Lookup(IDFormatFlagName) == nil {
		flags.String(IDFormatFlagName, "", fmt.Sprintf(`Configure UUID rendering in static text-table ID columns.
- Config path: [ %s ]
- Allowed    : [ compact|full ]
- Default    : [ compact ]`, IDFormatConfigPath))
	}
}

func BindFlags(cfg config.Hook, flags *pflag.FlagSet) error {
	if cfg == nil || flags == nil {
		return nil
	}
	bindings := []struct {
		flag       string
		configPath string
	}{
		{LayoutFlagName, LayoutConfigPath},
		{IDFormatFlagName, IDFormatConfigPath},
	}
	for _, binding := range bindings {
		if flag := flags.Lookup(binding.flag); flag != nil {
			if err := cfg.BindFlag(binding.configPath, flag); err != nil {
				return err
			}
		}
	}
	return nil
}

func Resolve(cmd *cobra.Command, cfg config.Hook, outputFormat string) (Settings, error) {
	settings := Settings{Layout: DefaultLayout, IDFormat: DefaultIDFormat}
	if outputFormat != cmdcommon.TEXT.String() {
		if explicitlyConfigured(cmd) {
			return settings, fmt.Errorf(
				"--%s and --%s are only supported with --%s text",
				LayoutFlagName,
				IDFormatFlagName,
				cmdcommon.OutputFlagName,
			)
		}
		return settings, nil
	}

	if cfg == nil {
		return settings, nil
	}

	layout := strings.ToLower(strings.TrimSpace(cfg.GetString(LayoutConfigPath)))
	if layout != "" {
		settings.Layout = Layout(layout)
	}
	idFormat := strings.ToLower(strings.TrimSpace(cfg.GetString(IDFormatConfigPath)))
	if idFormat != "" {
		settings.IDFormat = IDFormat(idFormat)
	}
	if err := settings.Validate(); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func (s Settings) Validate() error {
	switch s.Layout {
	case LayoutCompact, LayoutAuto, LayoutWide:
	default:
		return fmt.Errorf("invalid %s value %q, must be one of [compact auto wide]", LayoutConfigPath, s.Layout)
	}
	switch s.IDFormat {
	case IDFormatCompact, IDFormatFull:
	default:
		return fmt.Errorf("invalid %s value %q, must be one of [compact full]", IDFormatConfigPath, s.IDFormat)
	}
	return nil
}

func explicitlyConfigured(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	root := cmd.Root()
	return cmdcommon.CommandTreeFlagChanged(root, LayoutFlagName) ||
		cmdcommon.CommandTreeFlagChanged(root, IDFormatFlagName)
}
