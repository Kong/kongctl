package accesstoken

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "access-token"
)

var (
	accessTokenUse   = CommandName
	accessTokenShort = i18n.T(
		"root.products.konnect.systemaccount.accesstoken.accessTokenShort",
		"Manage Konnect system account access tokens")
	accessTokenLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.systemaccount.accesstoken.accessTokenLong",
		`The access-token command allows you to work with Konnect system account access tokens.`))
	accessTokenExample = normalizers.Examples(i18n.T(
		"root.products.konnect.systemaccount.accesstoken.accessTokenExamples",
		fmt.Sprintf(`
# List all access tokens for a system account
%[1]s get system-account access-token <account-id|name>
# Get a specific access token
%[1]s get system-account access-token <account-id|name> <token-id>
# Create an access token
%[1]s create system-account access-token <account-id|name> --name "my-token" --expires-at "2026-12-31T00:00:00Z"
`, meta.CLIName)))
)

func NewAccessTokenCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     accessTokenUse,
		Short:   accessTokenShort,
		Long:    accessTokenLong,
		Example: accessTokenExample,
		Aliases: []string{
			"accesstoken", "accesstokens", "access-tokens",
			"access_token", "access_tokens",
			"at", "ats", "AT", "ATS",
		},
	}

	if verb == verbs.Get || verb == verbs.List {
		return newGetAccessTokenCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}
	if verb == verbs.Create {
		return newCreateAccessTokenCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}
	if verb == verbs.Delete {
		return newDeleteAccessTokenCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}

	return &baseCmd, nil
}
