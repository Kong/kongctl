package identity

import (
	"fmt"
	"io"
	"text/tabwriter"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newTrustedIDPReadCommand(verb verbs.VerbValue, principals bool,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	command := &cobra.Command{
		Use: "trusted-idp [id]", Aliases: []string{"trusted-idps", "trusted_idps"},
		Short: "List or get directory Trusted IdP configuration",
		Long: "Inspect an enabled directory Token Vault's trusted identity providers. " +
			"Select one by UUID or exact --issuer-url; display names are not unique. " +
			"This is directory trust configuration, separate from auth-server identity providers.",
		PreRunE: parentPreRun,
	}
	command.Flags().String("directory-id", "", "Parent directory UUID")
	command.Flags().String("directory-name", "", "Exact parent directory name")
	command.Flags().String("issuer-url", "", "Exact issuer URL of the Trusted IdP")
	command.MarkFlagsOneRequired("directory-id", "directory-name")
	command.MarkFlagsMutuallyExclusive("directory-id", "directory-name")
	if principals {
		command.Use = "trusted-idp-principals"
		command.Aliases = []string{"trusted-idp-principal", "trusted_idp_principals"}
		command.Short = "List principals associated with a directory Trusted IdP"
		command.Long = "Inspect principals whose OIDC identities match the selected Trusted IdP. " +
			"Visibility does not imply administrative ownership or that the principal was provisioned by this IdP."
		command.Flags().String("trusted-idp-id", "", "Parent Trusted IdP UUID")
		command.MarkFlagsOneRequired("trusted-idp-id", "issuer-url")
		command.MarkFlagsMutuallyExclusive("trusted-idp-id", "issuer-url")
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		issuer, _ := cmd.Flags().GetString("issuer-url")
		if cmd.Flags().Changed("issuer-url") && issuer == "" {
			return fmt.Errorf("issuer-url must not be empty")
		}
		if principals {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			id, _ := cmd.Flags().GetString("trusted-idp-id")
			if (id == "") == (issuer == "") {
				return fmt.Errorf("provide exactly one of --trusted-idp-id or --issuer-url")
			}
			if id != "" && !util.IsValidUUID(id) {
				return fmt.Errorf("trusted-idp-id must be a UUID")
			}
			return nil
		}
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		if len(args) == 1 && issuer != "" {
			return fmt.Errorf("provide a Trusted IdP ID argument or --issuer-url, not both")
		}
		if len(args) == 1 && !util.IsValidUUID(args[0]) {
			return fmt.Errorf("trusted IdP ID argument must be a UUID; use --issuer-url for an exact issuer lookup")
		}
		return nil
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(cmd, args)
		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}
		logger, err := helper.GetLogger()
		if err != nil {
			return err
		}
		sdk, err := helper.GetKonnectSDK(cfg, logger)
		if err != nil {
			return err
		}
		format, err := helper.GetOutputFormat()
		if err != nil {
			return err
		}
		pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
		ctx := helper.GetContext()
		directory, err := groupDirectoryIDFromFlags(ctx, cmd, sdk.GetIdentityDirectoriesAPI(), pageSize)
		if err != nil {
			return err
		}
		api := sdk.GetIdentityTrustedIDPsAPI()
		issuer, _ := cmd.Flags().GetString("issuer-url")
		var selected *helpers.IdentityTrustedIDP
		if issuer != "" {
			selected, err = helpers.ResolveIdentityTrustedIDPIssuer(ctx, api, directory, issuer, pageSize)
		} else if principals {
			id, _ := cmd.Flags().GetString("trusted-idp-id")
			selected, err = helpers.GetIdentityTrustedIDP(ctx, api, directory, id)
		} else if len(args) == 1 {
			selected, err = helpers.GetIdentityTrustedIDP(ctx, api, directory, args[0])
		}
		if err != nil {
			return err
		}
		printer, err := cli.Format(format.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		if principals {
			values, err := helpers.ListIdentityTrustedIDPPrincipals(ctx, api, directory, selected.ID, pageSize)
			if err != nil {
				return err
			}
			records := make([]principalDisplayRecord, 0, len(values))
			for _, value := range values {
				records = append(records, principalDisplay(value))
			}
			return tableview.RenderForFormat(helper, false, format, printer, helper.GetStreams(), records, values, "")
		}
		if selected != nil {
			if format == cmdCommon.TEXT {
				return printTrustedIDPRows(helper.GetStreams().Out, []helpers.IdentityTrustedIDP{*selected})
			}
			return tableview.RenderForFormat(helper, false, format, printer, helper.GetStreams(),
				trustedIDPDisplay(*selected), selected, "")
		}
		values, err := helpers.ListIdentityTrustedIDPs(ctx, api, directory, pageSize)
		if err != nil {
			return err
		}
		if format == cmdCommon.TEXT {
			return printTrustedIDPRows(helper.GetStreams().Out, values)
		}
		records := make([]trustedIDPDisplayRecord, 0, len(values))
		for _, value := range values {
			records = append(records, trustedIDPDisplay(value))
		}
		return tableview.RenderForFormat(helper, false, format, printer, helper.GetStreams(), records, values, "")
	}
	return command
}

type trustedIDPDisplayRecord struct {
	ID, IssuerURL, DisplayName, JITProvisioningEnabled, VaultAccessEnabled string
}

func trustedIDPDisplay(value helpers.IdentityTrustedIDP) trustedIDPDisplayRecord {
	result := trustedIDPDisplayRecord{
		ID: value.ID, IssuerURL: value.IssuerURL,
		JITProvisioningEnabled: "unknown", VaultAccessEnabled: "unknown",
	}
	if value.DisplayName != nil {
		result.DisplayName = *value.DisplayName
	}
	if value.JitProvisioningEnabled != nil {
		result.JITProvisioningEnabled = fmt.Sprint(*value.JitProvisioningEnabled)
	}
	if value.VaultAccessEnabled != nil {
		result.VaultAccessEnabled = fmt.Sprint(*value.VaultAccessEnabled)
	}
	return result
}

func printTrustedIDPRows(out io.Writer, values []helpers.IdentityTrustedIDP) error {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	header := "ID\tISSUER_URL\tDISPLAY_NAME\tJIT_PROVISIONING_ENABLED\tVAULT_ACCESS_ENABLED"
	if _, err := fmt.Fprintln(writer, header); err != nil {
		return err
	}
	for _, value := range values {
		row := trustedIDPDisplay(value)
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", row.ID, row.IssuerURL, row.DisplayName,
			row.JITProvisioningEnabled, row.VaultAccessEnabled); err != nil {
			return err
		}
	}
	return writer.Flush()
}
