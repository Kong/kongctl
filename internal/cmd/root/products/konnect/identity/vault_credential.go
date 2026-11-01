package identity

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newVaultCredentialReadCommand(verb verbs.VerbValue, principal bool,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use:     "vault-provider-credential [credential-id]",
		Aliases: []string{"vault-provider-credentials", "provider-credential", "provider-credentials"},
		Short:   "Inspect public shared or user credential metadata under a Vault provider",
		Long: "Inspect credential IDs, routing metadata, status and timestamps under an explicit provider. " +
			"Credential values and enrollment URLs are excluded. Inspection does not issue OAuth enrollment URLs.",
		PreRunE: parentPreRun,
	}
	command.Flags().String("directory-id", "", "Parent directory UUID")
	command.Flags().String("directory-name", "", "Exact parent directory name")
	command.MarkFlagsOneRequired("directory-id", "directory-name")
	command.MarkFlagsMutuallyExclusive("directory-id", "directory-name")
	if principal {
		command.Use = "principal-vault-credential [credential-id]"
		command.Aliases = []string{"principal-vault-credentials", "vault-credential", "vault-credentials"}
		command.Short = "Inspect Token Vault authorizations held by an explicit principal"
		command.Long = "Inspect public provider, template, status, expiry and timestamps " +
			"for a principal's Vault authorizations. " +
			"Enrollment is a runtime operation; inspection grants no credential ownership or synchronization authority."
		command.Flags().String("principal-id", "", "Parent principal UUID")
		if err := command.MarkFlagRequired("principal-id"); err != nil {
			return nil, err
		}
	} else {
		command.Flags().String("provider-id", "", "Parent provider UUID")
		command.Flags().String("provider-name", "", "Exact parent provider name")
		command.MarkFlagsOneRequired("provider-id", "provider-name")
		command.MarkFlagsMutuallyExclusive("provider-id", "provider-name")
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		if len(args) == 1 && !util.IsValidUUID(args[0]) {
			return fmt.Errorf("credential ID must be a UUID")
		}
		if principal {
			id, _ := cmd.Flags().GetString("principal-id")
			if !util.IsValidUUID(id) {
				return fmt.Errorf("principal-id must be a UUID")
			}
		} else if _, err := groupParentSelector(cmd, "provider"); err != nil {
			return err
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
		ctx := helper.GetContext()
		pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
		directory, err := groupDirectoryIDFromFlags(ctx, cmd, sdk.GetIdentityDirectoriesAPI(), pageSize)
		if err != nil {
			return err
		}
		var data any
		var rows [][5]string
		found := len(args) == 0
		if principal {
			id, _ := cmd.Flags().GetString("principal-id")
			values, err := helpers.ListIdentityPrincipalVaultCredentials(
				ctx,
				sdk.GetIdentityPrincipalVaultCredentialsReader(),
				directory,
				id,
				pageSize,
			)
			if err != nil {
				return err
			}
			data = values
			for _, value := range values {
				if len(args) == 1 && value.ID != args[0] {
					continue
				}
				found = true
				if len(args) == 1 {
					data = value
				}
				rows = append(
					rows,
					[5]string{
						value.ID,
						value.Provider.Name,
						value.TemplateName,
						value.Status,
						basicAuthPasswordExpiration(value.ExpiresAt),
					},
				)
			}
		} else {
			id, _ := cmd.Flags().GetString("provider-id")
			name, _ := cmd.Flags().GetString("provider-name")
			if name != "" {
				value, err := helpers.ResolveIdentityVaultProviderName(
					ctx, sdk.GetIdentityVaultProvidersReader(), directory, name, pageSize,
				)
				if err != nil {
					return err
				}
				id = value.ID
			}
			values, err := helpers.ListIdentityVaultProviderCredentials(
				ctx, sdk.GetIdentityVaultProviderCredentialsReader(), directory, id, pageSize,
			)
			if err != nil {
				return err
			}
			data = values
			for _, value := range values {
				if len(args) == 1 && value.ID != args[0] {
					continue
				}
				found = true
				if len(args) == 1 {
					data = value
				}
				priority, subject := "-", "-"
				if value.CredentialType == "shared" {
					priority = "default"
				}
				if value.Priority != nil {
					priority = strconv.FormatInt(*value.Priority, 10)
				}
				if value.Principal != nil {
					subject = value.Principal.ID
				}
				rows = append(rows, [5]string{value.ID, value.CredentialType, value.Status, priority, subject})
			}
		}
		if !found {
			return fmt.Errorf("credential %q not found in the selected parent collection", args[0])
		}
		if format == cmdCommon.TEXT {
			return printVaultCredentialRows(helper.GetStreams().Out, rows, principal)
		}
		printer, err := cli.Format(format.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(data)
		return nil
	}
	return command, nil
}

func printVaultCredentialRows(out io.Writer, rows [][5]string, principal bool) error {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	header := "ID\tCREDENTIAL_TYPE\tSTATUS\tPRIORITY\tPRINCIPAL_ID"
	if principal {
		header = "ID\tPROVIDER_NAME\tTEMPLATE_NAME\tSTATUS\tEXPIRES_AT"
	}
	if _, err := fmt.Fprintln(writer, header); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", row[0], row[1], row[2], row[3], row[4]); err != nil {
			return err
		}
	}
	return writer.Flush()
}
