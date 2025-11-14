package regions

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "regions"
)

var (
	regionsUse = CommandName

	regionsShort = i18n.T("root.products.konnect.regions.regionsShort",
		"List available Konnect regions")

	regionsLong = normalizers.LongDesc(i18n.T("root.products.konnect.regions.regionsLong",
		`The regions command lists the Konnect regions that can be used for regional API requests.`))

	regionsExample = normalizers.Examples(i18n.T("root.products.konnect.regions.regionsExample",
		fmt.Sprintf(`
	# List Konnect regions
	%[1]s get regions
	`, meta.CLIName)))
)

func NewRegionsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     regionsUse,
		Short:   regionsShort,
		Long:    regionsLong,
		Example: regionsExample,
	}

	if verb == verbs.Get {
		return newGetRegionsCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}
	return &baseCmd, nil
}
