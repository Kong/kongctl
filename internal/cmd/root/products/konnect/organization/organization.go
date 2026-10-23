package organization

import (
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/organization/systemaccount"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "organization"
)

var (
	organizationUse = CommandName

	organizationShort = i18n.T("root.products.konnect.organization.organizationShort",
		"Get current organization information")

	organizationLong = normalizers.LongDesc(i18n.T("root.products.konnect.organization.organizationLong",
		`The organization command retrieves data about the currently authenticated Konnect organization.`))
)

func NewOrganizationCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     organizationUse,
		Short:   organizationShort,
		Long:    organizationLong,
		Aliases: []string{"org", "orgs"},
	}

	// Handle supported verbs
	if verb == verbs.Get || verb == verbs.List {
		return newGetOrganizationCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}
	if verb == verbs.Adopt {
		return newAdoptOrganizationCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}
	if verb == verbs.Create || verb == verbs.Delete {
		baseCmd.Short = "Manage Konnect organization resources"
		baseCmd.Long = "Manage resources within the currently authenticated Konnect organization."
		if parentPreRun != nil {
			baseCmd.PreRunE = parentPreRun
		}
		if addParentFlags != nil {
			addParentFlags(verb, &baseCmd)
		}

		systemAccountCmd, err := systemaccount.NewSystemAccountCmd(verb, addParentFlags, parentPreRun)
		if err != nil {
			return nil, err
		}
		baseCmd.AddCommand(systemAccountCmd)
		return &baseCmd, nil
	}

	// Return base command for unsupported verbs
	return &baseCmd, nil
}
