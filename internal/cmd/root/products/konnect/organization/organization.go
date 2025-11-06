package organization

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
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

	organizationExample = normalizers.Examples(
		i18n.T("root.products.konnect.organization.organizationExamples",
			fmt.Sprintf(`
	# Get current organization information
	%[1]s get organization
	# Get current organization information using explicit product
	%[1]s get konnect organization
	`, meta.CLIName)))
)

func NewOrganizationCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     organizationUse,
		Short:   organizationShort,
		Long:    organizationLong,
		Example: organizationExample,
		Aliases: []string{"org", "orgs"},
	}

	switch verb {
	case verbs.Get:
		return newGetOrganizationCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.List,
		verbs.Delete,
		verbs.Create,
		verbs.Add,
		verbs.Apply,
		verbs.Dump,
		verbs.Update,
		verbs.Help,
		verbs.Login,
		verbs.Plan,
		verbs.Sync,
		verbs.Diff,
		verbs.Export,
		verbs.Adopt,
		verbs.API,
		verbs.Kai,
		verbs.View:
		return &baseCmd, nil
	}

	return &baseCmd, nil
}
