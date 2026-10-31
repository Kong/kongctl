package identity

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newAuthServerSecretMutationCommand(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	command := &cobra.Command{
		Use: "auth-server-client-secret <secret-id>", Aliases: []string{"auth-server-client-secrets"},
		Short: fmt.Sprintf("%s an OAuth client secret", verb), PreRunE: parentPreRun,
	}
	command.Flags().String("auth-server-id", "", "Parent authorization server UUID")
	command.Flags().String("auth-server-name", "", "Exact parent authorization server name")
	command.Flags().String("client-id", "", "Parent OAuth client ID")
	command.Flags().String("client-name", "", "Exact parent OAuth client name")
	command.MarkFlagsOneRequired("auth-server-id", "auth-server-name")
	command.MarkFlagsMutuallyExclusive("auth-server-id", "auth-server-name")
	command.MarkFlagsOneRequired("client-id", "client-name")
	command.MarkFlagsMutuallyExclusive("client-id", "client-name")
	//exhaustive:ignore
	switch verb {
	case verbs.Create:
		command.Use = "auth-server-client-secret"
		command.Short = "Create an OAuth client secret and capture its value in a new private file"
		command.Flags().String("secret-output-file", "",
			"New file for generated or imported material (0600; never overwritten)")
		command.Flags().String("secret-env", "", "Environment variable containing a secret to import; omit to generate")
		command.Flags().String("secret-file", "", "File containing a secret to import; omit to generate")
		command.MarkFlagsMutuallyExclusive("secret-env", "secret-file")
		_ = command.MarkFlagRequired("secret-output-file")
		command.Flags().Bool("enabled", true, "Whether the new secret can authenticate")
	case verbs.Patch:
		command.Short = "Enable or disable an OAuth client secret"
		command.Flags().Bool("enabled", true, "Whether the secret can authenticate")
		_ = command.MarkFlagRequired("enabled")
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if verb == verbs.Create {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
		} else {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}
			if !util.IsValidUUID(args[0]) {
				return fmt.Errorf("secret-id must be a UUID")
			}
		}
		id, _ := cmd.Flags().GetString("client-id")
		name, _ := cmd.Flags().GetString("client-name")
		if (id == "") == (name == "") {
			return fmt.Errorf("provide exactly one of --client-id or --client-name")
		}
		return nil
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		return runAuthServerSecretMutation(cmd, args, verb)
	}
	return command
}

func runAuthServerSecretMutation(command *cobra.Command, args []string, verb verbs.VerbValue) error {
	helper := cmdpkg.BuildHelper(command, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	format, err := helper.GetOutputFormat()
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
	ctx := helper.GetContext()
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	serverID, err := authServerIDFromFlags(ctx, command, sdk.GetIdentityAuthServersAPI(), pageSize)
	if err != nil {
		return err
	}
	id, _ := command.Flags().GetString("client-id")
	name, _ := command.Flags().GetString("client-name")
	selector := id
	if name != "" {
		selector = name
	}
	api := sdk.GetIdentityAuthServerClientsAPI()
	client, err := helpers.ResolveIdentityAuthServerClient(ctx, api, serverID, selector, name != "", pageSize)
	if err != nil {
		return err
	}
	var data any
	//exhaustive:ignore
	switch verb {
	case verbs.Create:
		material, err := identitySecretSource(command)
		if err != nil {
			return err
		}
		enabled, _ := command.Flags().GetBool("enabled")
		path, _ := command.Flags().GetString("secret-output-file")
		data, err = helpers.CreateAndCaptureIdentityClientSecret(ctx, api, serverID, client.ID,
			kkComps.CreateSecretPayload{Secret: material, Enabled: &enabled}, path)
		if err != nil {
			return err
		}
	case verbs.Patch:
		enabled, _ := command.Flags().GetBool("enabled")
		data, err = helpers.UpdateIdentityClientSecret(ctx, api, serverID, client.ID, args[0], enabled)
		if err != nil {
			return err
		}
	case verbs.Delete:
		if _, err := helpers.GetIdentityClientSecret(ctx, api, serverID, client.ID, args[0]); err != nil {
			return err
		}
		if err := cmdpkg.ConfirmDelete(helper, fmt.Sprintf("OAuth client secret %q", args[0])); err != nil {
			return err
		}
		if err := helpers.DeleteIdentityClientSecret(ctx, api, serverID, client.ID, args[0]); err != nil {
			return err
		}
		data = map[string]string{"id": args[0], "status": "deleted"}
	default:
		return fmt.Errorf("unsupported client-secret operation %q", verb)
	}
	if format == cmdCommon.TEXT {
		encoded, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(helper.GetStreams().Out, string(encoded))
		return err
	}
	printer, err := cli.Format(format.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	printer.Print(data)
	return nil
}

func identitySecretSource(command *cobra.Command) (*string, error) {
	variable, _ := command.Flags().GetString("secret-env")
	file, _ := command.Flags().GetString("secret-file")
	if variable == "" && file == "" {
		if command.Flags().Changed("secret-env") || command.Flags().Changed("secret-file") {
			return nil, fmt.Errorf("secret source must not be empty")
		}
		return nil, nil
	}
	source := tags.SecretSource{Kind: "env", Reference: variable}
	if file != "" {
		path, err := filepath.Abs(file)
		if err != nil {
			return nil, fmt.Errorf("resolve secret file path: %w", err)
		}
		source = tags.SecretSource{Kind: "file", Reference: path, RootDir: filepath.Dir(path)}
	}
	value, err := tags.ResolveSecretExpression(tags.SecretExpression{
		Parts: []tags.SecretPart{{Source: &source}},
	})
	if err != nil {
		return nil, fmt.Errorf("could not resolve %s secret source %q; verify that it contains a non-empty string",
			source.Kind, source.Reference)
	}
	return &value, nil
}
