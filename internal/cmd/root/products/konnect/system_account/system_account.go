package systemaccount

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "system_account"
)

var (
	systemAccountUse   = CommandName
	systemAccountShort = i18n.T("root.products.konnect.systemAccount.systemAccountShort", "Manage Konnect systemAccount resources")
	systemAccountLong  = normalizers.LongDesc(i18n.T("root.products.konnect.systemAccount.systemAccountLong",
		`The systemAccount command allows you to work with Konnect systemAccount resources.`))
	systemAccountExample = normalizers.Examples(i18n.T("root.products.konnect.systemAccount.systemAccountExamples",
		fmt.Sprintf(`
# List all system_accounts
%[1]s get system accounts
# Get a specific system account
%[1]s get system_account <id|name>
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
		Aliases: []string{"systemAccount", "systemAccounts", "system-account", "system-accounts", "sa", "system_accounts"},
	}

	switch verb {
	case verbs.Get:
		return newGetSystemAccountCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.List:
		return newGetSystemAccountCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	default:
		return &baseCmd, nil
	}
}
