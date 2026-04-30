package accesstoken

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	deleteAccessTokenShort = i18n.T(
		"root.products.konnect.systemaccount.accesstoken.deleteAccessTokenShort",
		"Delete a system account access token")
	deleteAccessTokenLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.systemaccount.accesstoken.deleteAccessTokenLong",
		`Delete an access token belonging to a Konnect system account.`))
	deleteAccessTokenExample = normalizers.Examples(i18n.T(
		"root.products.konnect.systemaccount.accesstoken.deleteAccessTokenExamples",
		fmt.Sprintf(`
# Delete an access token
%[1]s delete system-account access-token <account-id|name> <token-id>
`, meta.CLIName)))
)

type deleteAccessTokenCmd struct {
	*cobra.Command
}

func newDeleteAccessTokenCmd(
	verb verbs.VerbValue,
	base *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *deleteAccessTokenCmd {
	c := &deleteAccessTokenCmd{
		Command: &cobra.Command{
			Use:     fmt.Sprintf("%s [account-id|name] [token-id]", CommandName),
			Short:   deleteAccessTokenShort,
			Long:    deleteAccessTokenLong,
			Example: deleteAccessTokenExample,
			Aliases: base.Aliases,
			Args:    cobra.ExactArgs(2),
		},
	}

	if addParentFlags != nil {
		addParentFlags(verb, c.Command)
	}

	c.PreRunE = func(cobraCmd *cobra.Command, args []string) error {
		if parentPreRun != nil {
			return parentPreRun(cobraCmd, args)
		}
		return nil
	}
	c.RunE = c.runE

	return c
}

func (dc *deleteAccessTokenCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)

	accountIDOrName := args[0]
	tokenID := args[1]

	if err := cmd.ConfirmDelete(helper, fmt.Sprintf("access token %q", tokenID)); err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	accountID, err := resolveSystemAccountID(accountIDOrName, sdk, helper, cfg)
	if err != nil {
		return err
	}

	res, err := sdk.GetSystemAccountAccessTokenAPI().DeleteSystemAccountAccessToken(
		helper.GetContext(), accountID, tokenID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to delete access token", err, helper.GetCmd(), attrs...)
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	printer.Print(res)

	return nil
}
