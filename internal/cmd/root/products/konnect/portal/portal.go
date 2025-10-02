package portal

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "portal"
)

var (
	portalUse   = CommandName
	portalShort = i18n.T("root.products.konnect.portal.portalShort",
		"Manage Konnect portal resources")
	portalLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.portalLong",
		`The portal command allows you to work with Konnect portal resources.`))
	portalExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.portalExamples",
			fmt.Sprintf(`
# List all the Konnect portals for the organization
%[1]s get portals
# Get a specific Konnect portal
%[1]s get portal <id|name>
# List portal pages
%[1]s get portal pages --portal-id <portal-id>
# List portal applications
%[1]s get portal applications --portal-id <portal-id>
# List portals using explicit konnect product
%[1]s get konnect portals
`, meta.CLIName)))
)

func NewPortalCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     portalUse,
		Short:   portalShort,
		Long:    portalLong,
		Example: portalExample,
		Aliases: []string{"portals", "p", "ps", "P", "PS"},
	}

	switch verb {
	case verbs.Get:
		return newGetPortalCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.List:
		return newGetPortalCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.Delete:
		return newDeletePortalCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.Create, verbs.Add, verbs.Apply, verbs.Dump, verbs.Update, verbs.Help, verbs.Login,
		verbs.Plan, verbs.Sync, verbs.Diff, verbs.Export, verbs.Adopt:
		return &baseCmd, nil
	}

	return &baseCmd, nil
}
