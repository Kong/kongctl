package systemaccount

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/token"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "system-account"
)

var (
	systemAccountUse   = CommandName
	systemAccountShort = i18n.T("root.products.konnect.systemaccount.systemAccountShort",
		"Manage Konnect system account resources")
	systemAccountLong = normalizers.LongDesc(i18n.T("root.products.konnect.systemaccount.systemAccountLong",
		`The systemAccount command allows you to work with Konnect system account resources.`))
	systemAccountExample = normalizers.Examples(i18n.T("root.products.konnect.systemaccount.systemAccountExamples",
		fmt.Sprintf(`
# List all system-accounts
%[1]s get system-accounts
# Get a specific system account
%[1]s get system-account <id|name>
`, meta.CLIName)))
)

func NewSystemAccountCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     systemAccountUse,
		Short:   systemAccountShort,
		Long:    systemAccountLong,
		Example: systemAccountExample,
		Aliases: []string{
			"systemaccount", "systemaccounts", "system-accounts", "system_account", "system_accounts",
			"sa", "sas", "SA", "SAS",
		},
	}

	// Handle supported verbs
	if verb == verbs.Get || verb == verbs.List {
		cmd := newGetSystemAccountCmd(verb, &baseCmd, addParentFlags, parentPreRun)
		cmd.AddCommand(newGetSystemAccountRolesCmd(verb, addParentFlags, parentPreRun))
		cmd.AddCommand(newGetSystemAccountTeamsCmd(verb, addParentFlags, parentPreRun))
		spatCmd, err := token.NewSPATCmd(verb, addParentFlags, parentPreRun)
		if err != nil {
			return nil, err
		}
		if spatCmd != nil {
			cmd.AddCommand(spatCmd)
		}
		return cmd.Command, nil
	}
	if verb == verbs.Create || verb == verbs.Delete {
		if parentPreRun != nil {
			baseCmd.PreRunE = parentPreRun
		}
		if addParentFlags != nil {
			addParentFlags(verb, &baseCmd)
		}
		spatCmd, err := token.NewSPATCmd(verb, addParentFlags, parentPreRun)
		if err != nil {
			return nil, err
		}
		baseCmd.AddCommand(spatCmd)
		return &baseCmd, nil
	}

	// Return base command for unsupported verbs
	return &baseCmd, nil
}
