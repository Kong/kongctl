package dcrprovider

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "dcr-provider"
)

var (
	dcrProviderUse   = CommandName
	dcrProviderShort = i18n.T("root.products.konnect.dcrprovider.dcrProviderShort",
		"Manage Konnect DCR provider resources")
	dcrProviderLong = normalizers.LongDesc(i18n.T("root.products.konnect.dcrprovider.dcrProviderLong",
		`The dcr-provider command allows you to work with Konnect Dynamic Client Registration provider resources.`))
	dcrProviderExample = normalizers.Examples(
		i18n.T("root.products.konnect.dcrprovider.dcrProviderExamples",
			fmt.Sprintf(`
	# List all the Konnect DCR providers for the organization
	%[1]s get dcr-providers
	# Get a specific Konnect DCR provider
	%[1]s get dcr-provider <id|name>
	# List DCR providers using explicit konnect product
	%[1]s get konnect dcr-providers
	`, meta.CLIName)))
)

func NewDCRProviderCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     dcrProviderUse,
		Short:   dcrProviderShort,
		Long:    dcrProviderLong,
		Example: dcrProviderExample,
		Aliases: []string{"dcr-providers", "dcr-provider", "dcrp", "dcrps", "DCRP", "DCRPS"},
	}

	if verb == verbs.Get || verb == verbs.List {
		return newGetDCRProviderCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}

	return &baseCmd, nil
}
