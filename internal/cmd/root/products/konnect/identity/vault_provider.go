package identity

import (
	"fmt"
	"io"
	"strings"
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

func newVaultProviderReadCommand(verb verbs.VerbValue, templates bool,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	command := &cobra.Command{
		Use:     "vault-provider [id-or-name]",
		Aliases: []string{"vault-providers", "vault_providers", "credential-provider", "credential-providers"},
		Short:   "List or get directory Token Vault provider configuration",
		Long: "Inspect public Credential Provider configuration in an enabled directory Token Vault. " +
			"Select by UUID or exact name; --provider-name also selects names shaped like UUIDs. " +
			"Responses contain effective endpoints, not the stored override settings. " +
			"Provider client secrets, credential values and enrollment capabilities are excluded.",
		PreRunE: parentPreRun,
	}
	if templates {
		command.Use = "credential-provider-template [name]"
		command.Aliases = []string{"credential-provider-templates", "provider-template", "provider-templates"}
		command.Short = "List or get the built-in credential-provider template catalog"
		command.Long = "Inspect the read-only catalog of built-in Credential Provider templates. " +
			"Templates describe OAuth/static-secret behavior, endpoint defaults, scopes and PKCE requirements. " +
			"This catalog does not require a directory selector or enabled Token Vault and is not desired state."
	} else {
		command.Flags().String("directory-id", "", "Parent directory UUID")
		command.Flags().String("directory-name", "", "Exact parent directory name")
		command.Flags().String("provider-name", "", "Exact provider name, including names shaped like UUIDs")
		command.MarkFlagsOneRequired("directory-id", "directory-name")
		command.MarkFlagsMutuallyExclusive("directory-id", "directory-name")
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		if len(args) == 1 && args[0] == "" {
			return fmt.Errorf("provider selector must not be empty")
		}
		if templates {
			return nil
		}
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("provider-name")
		if cmd.Flags().Changed("provider-name") && name == "" {
			return fmt.Errorf("provider-name must not be empty")
		}
		if len(args) == 1 && name != "" {
			return fmt.Errorf("provide a provider argument or --provider-name, not both")
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
		api := sdk.GetIdentityVaultProvidersReader()
		printer, err := cli.Format(format.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		if templates {
			if len(args) == 1 {
				value, err := helpers.GetIdentityCredentialProviderTemplate(ctx, api, args[0])
				if err != nil {
					return err
				}
				if format == cmdCommon.TEXT {
					return printProviderTemplateRows(
						helper.GetStreams().Out,
						[]helpers.IdentityCredentialProviderTemplate{*value},
					)
				}
				return tableview.RenderForFormat(
					helper,
					false,
					format,
					printer,
					helper.GetStreams(),
					providerTemplateDisplay(*value),
					value,
					"",
				)
			}
			values, err := helpers.ListIdentityCredentialProviderTemplates(ctx, api, pageSize)
			if err != nil {
				return err
			}
			if format == cmdCommon.TEXT {
				return printProviderTemplateRows(helper.GetStreams().Out, values)
			}
			records := make([]providerTemplateDisplayRecord, 0, len(values))
			for _, value := range values {
				records = append(records, providerTemplateDisplay(value))
			}
			return tableview.RenderForFormat(helper, false, format, printer, helper.GetStreams(), records, values, "")
		}
		directory, err := groupDirectoryIDFromFlags(ctx, cmd, sdk.GetIdentityDirectoriesAPI(), pageSize)
		if err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("provider-name")
		var selected *helpers.IdentityVaultProvider
		if name != "" {
			selected, err = helpers.ResolveIdentityVaultProviderName(ctx, api, directory, name, pageSize)
		} else if len(args) == 1 {
			if util.IsValidUUID(args[0]) {
				selected, err = helpers.GetIdentityVaultProvider(ctx, api, directory, args[0])
			} else {
				selected, err = helpers.ResolveIdentityVaultProviderName(ctx, api, directory, args[0], pageSize)
			}
		}
		if err != nil {
			return err
		}
		if selected != nil {
			if format == cmdCommon.TEXT {
				return printVaultProviderRows(helper.GetStreams().Out, []helpers.IdentityVaultProvider{*selected})
			}
			return tableview.RenderForFormat(
				helper,
				false,
				format,
				printer,
				helper.GetStreams(),
				vaultProviderDisplay(*selected),
				selected,
				"",
			)
		}
		values, err := helpers.ListIdentityVaultProviders(ctx, api, directory, pageSize)
		if err != nil {
			return err
		}
		if format == cmdCommon.TEXT {
			return printVaultProviderRows(helper.GetStreams().Out, values)
		}
		records := make([]vaultProviderDisplayRecord, 0, len(values))
		for _, value := range values {
			records = append(records, vaultProviderDisplay(value))
		}
		return tableview.RenderForFormat(helper, false, format, printer, helper.GetStreams(), records, values, "")
	}
	return command
}

type vaultProviderDisplayRecord struct{ ID, Name, TemplateName, CredentialType, Enabled, DCR, PKCE string }

func vaultProviderDisplay(value helpers.IdentityVaultProvider) vaultProviderDisplayRecord {
	return vaultProviderDisplayRecord{
		ID: value.ID, Name: value.Name, TemplateName: value.TemplateName,
		CredentialType: value.CredentialType, Enabled: authChildBool(value.Enabled), DCR: authChildBool(value.DCREnabled),
		PKCE: authChildBool(value.PKCEEnabled),
	}
}

func printVaultProviderRows(out io.Writer, values []helpers.IdentityVaultProvider) error {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	header := "ID\tNAME\tTEMPLATE_NAME\tCREDENTIAL_TYPE\tENABLED\tDCR_ENABLED\tPKCE_ENABLED"
	if _, err := fmt.Fprintln(writer, header); err != nil {
		return err
	}
	for _, value := range values {
		v := vaultProviderDisplay(value)
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", v.ID, v.Name, v.TemplateName,
			v.CredentialType, v.Enabled, v.DCR, v.PKCE); err != nil {
			return err
		}
	}
	return writer.Flush()
}

type providerTemplateDisplayRecord struct{ Name, DisplayName, SecretType, BearerMethods, PKCERequired string }

func providerTemplateDisplay(value helpers.IdentityCredentialProviderTemplate) providerTemplateDisplayRecord {
	return providerTemplateDisplayRecord{
		Name: value.Name, DisplayName: value.DisplayName, SecretType: value.SecretType,
		BearerMethods: strings.Join(value.BearerMethodsSupported, ","), PKCERequired: authChildBool(value.PKCERequired),
	}
}

func printProviderTemplateRows(out io.Writer, values []helpers.IdentityCredentialProviderTemplate) error {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	header := "NAME\tDISPLAY_NAME\tSECRET_TYPE\tBEARER_METHODS_SUPPORTED\tPKCE_REQUIRED"
	if _, err := fmt.Fprintln(writer, header); err != nil {
		return err
	}
	for _, value := range values {
		v := providerTemplateDisplay(value)
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", v.Name, v.DisplayName, v.SecretType,
			v.BearerMethods, v.PKCERequired); err != nil {
			return err
		}
	}
	return writer.Flush()
}
