package api

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "api"
)

var (
	apiUse   = CommandName
	apiShort = i18n.T("root.products.konnect.api.apiShort",
		"Manage Konnect API resources")
	apiLong = normalizers.LongDesc(i18n.T("root.products.konnect.api.apiLong",
		`The api command allows you to work with Konnect API resources.`))
	apiExample = normalizers.Examples(
		i18n.T("root.products.konnect.api.apiExamples",
			fmt.Sprintf(`
	# List all the Konnect APIs for the organization
	%[1]s get apis
	# Get a specific Konnect API
	%[1]s get api <id|name>
	# List API documents for a specific API
	%[1]s get api documents --api-id <api-id>
	# List API versions for a specific API
	%[1]s get api versions --api-id <api-id>
	# List API publications for a specific API
	%[1]s get api publications --api-id <api-id>
	# List API implementations for a specific API
	%[1]s get api implementations --api-id <api-id>
	# List APIs using explicit konnect product
	%[1]s get konnect apis
	`, meta.CLIName)))
)

func NewAPICmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     apiUse,
		Short:   apiShort,
		Long:    apiLong,
		Example: apiExample,
		Aliases: []string{"apis", "a", "A"},
	}

	switch verb {
	case verbs.Get:
		return newGetAPICmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.List:
		return newGetAPICmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.Create, verbs.Delete, verbs.Add, verbs.Apply, verbs.Dump, verbs.Update, verbs.Help, verbs.Login,
<<<<<<< HEAD
		verbs.Plan, verbs.Sync, verbs.Diff, verbs.Export, verbs.Adopt, verbs.Ask:
=======
		verbs.Plan, verbs.Sync, verbs.Diff, verbs.Export, verbs.Ask, verbs.API:
>>>>>>> 00a7631 (feat: Adds api feature)
		return &baseCmd, nil
	}

	return &baseCmd, nil
}
