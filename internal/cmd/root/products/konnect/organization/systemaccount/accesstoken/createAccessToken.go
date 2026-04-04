package accesstoken

import (
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	CreateTokenNameFlagName      = "name"
	CreateTokenExpiresAtFlagName = "expires-at"
)

var (
	createAccessTokenShort = i18n.T(
		"root.products.konnect.systemaccount.accesstoken.createAccessTokenShort",
		"Create a new system account access token")
	createAccessTokenLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.systemaccount.accesstoken.createAccessTokenLong",
		`Create a new access token for a Konnect system account.
The token value is only displayed once upon creation; store it securely.`))
	createAccessTokenExample = normalizers.Examples(i18n.T(
		"root.products.konnect.systemaccount.accesstoken.createAccessTokenExamples",
		fmt.Sprintf(`
# Create an access token for a system account
%[1]s create system-account access-token <account-id|name> --name "ci-token" --expires-at "2027-01-01T00:00:00Z"
`, meta.CLIName)))
)

type createAccessTokenCmd struct {
	*cobra.Command
}

func newCreateAccessTokenCmd(
	verb verbs.VerbValue,
	base *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *createAccessTokenCmd {
	c := &createAccessTokenCmd{
		Command: &cobra.Command{
			Use:     fmt.Sprintf("%s [account-id|name]", CommandName),
			Short:   createAccessTokenShort,
			Long:    createAccessTokenLong,
			Example: createAccessTokenExample,
			Aliases: base.Aliases,
			Args:    cobra.ExactArgs(1),
		},
	}

	if addParentFlags != nil {
		addParentFlags(verb, c.Command)
	}

	c.Flags().String(CreateTokenNameFlagName, "",
		"Name for the new access token (required)")
	c.Flags().String(CreateTokenExpiresAtFlagName, "",
		"Expiration timestamp in RFC3339 format, e.g. 2027-01-01T00:00:00Z (required)")

	_ = c.MarkFlagRequired(CreateTokenNameFlagName)
	_ = c.MarkFlagRequired(CreateTokenExpiresAtFlagName)

	c.PreRunE = func(cobraCmd *cobra.Command, args []string) error {
		if parentPreRun != nil {
			return parentPreRun(cobraCmd, args)
		}
		return nil
	}
	c.RunE = c.runE

	return c
}

func (cc *createAccessTokenCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)

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

	accountID, err := resolveSystemAccountID(args[0], sdk, helper, cfg)
	if err != nil {
		return err
	}

	tokenName, _ := cobraCmd.Flags().GetString(CreateTokenNameFlagName)
	expiresAtStr, _ := cobraCmd.Flags().GetString(CreateTokenExpiresAtFlagName)

	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return fmt.Errorf("invalid --expires-at value %q: must be RFC3339 format (e.g. 2027-01-01T00:00:00Z): %w",
			expiresAtStr, err)
	}

	body := &kkComps.CreateSystemAccountAccessToken{
		Name:      tokenName,
		ExpiresAt: expiresAt,
	}

	res, err := sdk.GetSystemAccountAccessTokenAPI().CreateSystemAccountAccessToken(
		helper.GetContext(), accountID, body)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to create access token", err, helper.GetCmd(), attrs...)
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

	created := res.GetSystemAccountAccessTokenCreated()
	printer.Print(created)

	return nil
}
