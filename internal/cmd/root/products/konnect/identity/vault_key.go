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

func newVaultKeyReadCommand(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	command := &cobra.Command{
		Use: "vault-key [id]", Aliases: []string{"vault-keys", "vault_keys"},
		Short: "List or get directory Token Vault public encryption keys",
		Long: "Inspect an enabled directory Token Vault's public encryption keys. " +
			"Select one by UUID or exact --kid fingerprint; names are not unique. " +
			"These keys encrypt Vault data and are separate from principal/group API-key credentials. " +
			"JSON and YAML output include public key material; no private key is returned.",
		PreRunE: parentPreRun,
	}
	common.AddIdentityDirectoryFlags(command)
	command.Flags().String("kid", "", "Exact RFC 7638 public-key fingerprint")
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		kid, _ := cmd.Flags().GetString("kid")
		if cmd.Flags().Changed("kid") && kid == "" {
			return fmt.Errorf("kid must not be empty")
		}
		if len(args) == 1 && kid != "" {
			return fmt.Errorf("provide a Vault key ID argument or --kid, not both")
		}
		if len(args) == 1 && !util.IsValidUUID(args[0]) {
			return fmt.Errorf("vault key ID argument must be a UUID; use --kid for a fingerprint lookup")
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
		api := sdk.GetIdentityVaultKeysAPI()
		kid, _ := cmd.Flags().GetString("kid")
		var selected *helpers.IdentityVaultKey
		if kid != "" {
			selected, err = helpers.ResolveIdentityVaultKeyKid(ctx, api, directory, kid, pageSize)
		} else if len(args) == 1 {
			selected, err = helpers.GetIdentityVaultKey(ctx, api, directory, args[0])
		}
		if err != nil {
			return err
		}
		printer, err := cli.Format(format.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		if selected != nil {
			if format == cmdCommon.TEXT {
				return printVaultKeyRows(helper.GetStreams().Out, []helpers.IdentityVaultKey{*selected})
			}
			return tableview.RenderForFormat(helper, false, format, printer, helper.GetStreams(),
				vaultKeyDisplay(*selected), selected, "")
		}
		values, err := helpers.ListIdentityVaultKeys(ctx, api, directory, pageSize)
		if err != nil {
			return err
		}
		if format == cmdCommon.TEXT {
			return printVaultKeyRows(helper.GetStreams().Out, values)
		}
		records := make([]vaultKeyDisplayRecord, 0, len(values))
		for _, value := range values {
			records = append(records, vaultKeyDisplay(value))
		}
		return tableview.RenderForFormat(helper, false, format, printer, helper.GetStreams(), records, values, "")
	}
	return command
}

type vaultKeyDisplayRecord struct {
	ID, Name, Kid, Algorithm string
	KeySize                  int
}

func vaultKeyDisplay(value helpers.IdentityVaultKey) vaultKeyDisplayRecord {
	return vaultKeyDisplayRecord{
		ID: value.ID, Name: value.Name, Kid: value.Kid,
		Algorithm: value.Algorithm, KeySize: value.KeySize,
	}
}

func printVaultKeyRows(out io.Writer, values []helpers.IdentityVaultKey) error {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "ID\tNAME\tKID\tALGORITHM\tKEY_SIZE"); err != nil {
		return err
	}
	for _, value := range values {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%d\n", value.ID, value.Name, value.Kid,
			value.Algorithm, value.KeySize); err != nil {
			return err
		}
	}
	return writer.Flush()
}
