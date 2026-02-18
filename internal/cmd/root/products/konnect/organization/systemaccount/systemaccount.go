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
		return newGetSystemAccountCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}

	// Return base command for unsupported verbs
	return &baseCmd, nil
}
