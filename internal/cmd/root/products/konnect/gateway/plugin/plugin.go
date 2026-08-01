package plugin

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	pluginUse   = "plugin"
	pluginShort = i18n.T("root.products.konnect.gateway.plugin.pluginShort",
		"Manage Konnect Kong Gateway Plugins")
	pluginLong = normalizers.LongDesc(i18n.T("root.products.konnect.gateway.plugin.pluginLong",
		`The plugin command allows you to work with Konnect Kong Gateway Plugin resources.`))
	pluginExamples = normalizers.Examples(i18n.T("root.products.konnect.gateway.plugin.pluginExamples",
		fmt.Sprintf(`
	# List the Konnect Kong Gateway Plugins for a control plane
	%[1]s get konnect gateway control-plane plugins --control-plane-name <name>
	# Get a specific Konnect Kong Gateway Plugin in a control plane
	%[1]s get konnect gateway control-plane plugin <id|name> --control-plane-name <name>
	`, meta.CLIName)))
)

func NewPluginCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     pluginUse,
		Short:   pluginShort,
		Long:    pluginLong,
		Example: pluginExamples,
		Aliases: []string{"plugins"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return common.BindControlPlaneFlags(cmd, args)
		},
	}

	addFlagsFunc := func(verb verbs.VerbValue, cmd *cobra.Command) {
		common.AddControlPlaneFlags(cmd)
		if addParentFlags != nil {
			addParentFlags(verb, cmd)
		}
	}

	if verb == verbs.Get || verb == verbs.List {
		return newGetPluginCmd(verb, &baseCmd, addFlagsFunc, parentPreRun).Command, nil
	}

	return &baseCmd, nil
}
