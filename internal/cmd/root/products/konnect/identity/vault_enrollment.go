package identity

import (
	"context"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newVaultCredentialMutationCommand(verb verbs.VerbValue, enrollment bool,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use:     "vault-provider-credential",
		Aliases: []string{"vault-provider-credentials", "provider-credential", "provider-credentials"},
		Short:   "Create a pending shared OAuth credential and privately capture its enrollment URL", PreRunE: parentPreRun,
		Long: "Create a shared OAuth credential for an explicitly selected provider. " +
			"The enrollment URL is a private capability saved only in a new 0600 capture file. " +
			"Credential creation does not complete consent. Static credentials use declarative configuration. " +
			"These explicit operations do not stamp management metadata or adopt user authorizations.",
	}
	if enrollment {
		if verb != verbs.Create {
			return nil, fmt.Errorf("enrollment URL issuance requires create")
		}
		command.Use = "vault-provider-enrollment <credential-id>"
		command.Aliases = []string{"vault-provider-enrollments", "provider-enrollment", "provider-enrollments"}
		command.Short = "Issue an enrollment URL for an existing shared OAuth credential into a new private file"
		command.Long = "Explicitly issue a new private enrollment URL for an existing shared OAuth credential. " +
			"This invokes the API's capability-producing detail GET once. " +
			"Normal credential get/list commands never issue enrollment URLs. " +
			"Capture does not complete consent or prove that previous URLs remain valid."
	} else if verb == verbs.Delete {
		command.Use = "vault-provider-credential <credential-id>"
		command.Short = "Delete an explicitly selected shared Vault credential record"
		command.Long = "Delete one shared static or OAuth credential after proving its provider membership. " +
			"User authorizations are not deleted through this command. No enrollment URL is issued. " +
			"Remote record deletion does not prove upstream token invalidation."
	} else if verb != verbs.Create {
		return nil, fmt.Errorf("unsupported shared credential operation")
	}
	common.AddIdentityDirectoryFlags(command)
	command.Flags().String("provider-id", "", "Parent provider UUID")
	command.Flags().String("provider-name", "", "Exact parent provider name")
	command.MarkFlagsOneRequired("provider-id", "provider-name")
	command.MarkFlagsMutuallyExclusive("provider-id", "provider-name")
	if verb == verbs.Create {
		command.Flags().
			String("secret-output-file", "", "New private enrollment capture file (0600; never overwritten)")
		if err := command.MarkFlagRequired("secret-output-file"); err != nil {
			return nil, err
		}
		if !enrollment {
			command.Flags().String("selector", "", "CEL routing selector; omit with priority for the default slot")
			command.Flags().Int64("priority", 0, "Nonnegative routing priority, required with selector")
			command.MarkFlagsRequiredTogether("selector", "priority")
		}
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		if _, err := groupParentSelector(cmd, "provider"); err != nil {
			return err
		}
		if enrollment || verb == verbs.Delete {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}
			if !util.IsValidUUID(args[0]) {
				return fmt.Errorf("credential-id must be a UUID")
			}
		} else {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			if _, err := vaultCredentialRoutingFromFlags(cmd); err != nil {
				return err
			}
		}
		if verb == verbs.Create && cmd.Flags().Changed("secret-output-file") {
			path, _ := cmd.Flags().GetString("secret-output-file")
			if path == "" {
				return fmt.Errorf("secret-output-file must not be empty")
			}
		}
		return nil
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		return runVaultCredentialMutation(cmd, args, verb, enrollment)
	}
	return command, nil
}

func vaultCredentialRoutingFromFlags(command *cobra.Command) (helpers.IdentityVaultCredentialRouting, error) {
	var routing helpers.IdentityVaultCredentialRouting
	if command.Flags().Changed("selector") != command.Flags().Changed("priority") {
		return routing, fmt.Errorf("provide --selector and --priority together, or omit both for default")
	}
	if command.Flags().Changed("selector") {
		selector, _ := command.Flags().GetString("selector")
		priority, _ := command.Flags().GetInt64("priority")
		if selector == "" {
			return routing, fmt.Errorf("selector must not be empty")
		}
		routing.Selector, routing.Priority = &selector, &priority
	}
	return routing, routing.Validate()
}

func vaultCredentialScopeFromFlags(
	ctx context.Context,
	command *cobra.Command,
	sdk helpers.SDKAPI,
	pageSize int64,
) (helpers.IdentityVaultCredentialScope, error) {
	var scope helpers.IdentityVaultCredentialScope
	directory, err := groupDirectoryIDFromFlags(ctx, command, sdk.GetIdentityDirectoriesAPI(), pageSize)
	if err != nil {
		return scope, err
	}
	id, _ := command.Flags().GetString("provider-id")
	name, _ := command.Flags().GetString("provider-name")
	var provider *helpers.IdentityVaultProvider
	if name != "" {
		provider, err = helpers.ResolveIdentityVaultProviderName(
			ctx,
			sdk.GetIdentityVaultProvidersReader(),
			directory,
			name,
			pageSize,
		)
	} else {
		provider, err = helpers.GetIdentityVaultProvider(ctx, sdk.GetIdentityVaultProvidersReader(), directory, id)
	}
	if err != nil {
		return scope, err
	}
	return helpers.IdentityVaultCredentialScope{
		DirectoryID:  directory,
		ProviderID:   provider.ID,
		ProviderName: provider.Name,
	}, nil
}

func runVaultCredentialMutation(command *cobra.Command, args []string, verb verbs.VerbValue, enrollment bool) error {
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
	scope, err := vaultCredentialScopeFromFlags(ctx, command, sdk, int64(cfg.GetInt(common.RequestPageSizeConfigPath)))
	if err != nil {
		return err
	}
	api := sdk.GetIdentityVaultEnrollmentAPI()
	var value *helpers.IdentityVaultProviderCredential
	if verb == verbs.Delete {
		values, err := helpers.ListIdentityVaultProviderCredentials(ctx, api, scope.DirectoryID, scope.ProviderID, 100)
		if err != nil {
			return err
		}
		found := false
		for _, candidate := range values {
			if candidate.ID == args[0] && candidate.CredentialType == "shared" {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("shared credential is absent from the selected provider")
		}
		if err := cmdpkg.ConfirmDelete(helper, fmt.Sprintf("shared Vault credential %q", args[0])); err != nil {
			return err
		}
		if err := helpers.DeleteIdentitySharedVaultCredential(ctx, api, scope, args[0]); err != nil {
			return err
		}
		if format == cmdCommon.TEXT {
			_, err := fmt.Fprintf(helper.GetStreams().Out, "id: %s\nstatus: deleted\n", args[0])
			return err
		}
		printer, err := cli.Format(format.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(map[string]string{"id": args[0], "status": "deleted"})
		return nil
	}
	path, _ := command.Flags().GetString("secret-output-file")
	if enrollment {
		value, err = helpers.CaptureIdentityVaultEnrollment(ctx, api, scope, args[0], path)
	} else {
		routing, routingErr := vaultCredentialRoutingFromFlags(command)
		if routingErr != nil {
			return routingErr
		}
		value, err = helpers.CreateAndCaptureIdentityVaultEnrollment(ctx, api, scope, routing, path)
	}
	if err != nil {
		return err
	}
	if format == cmdCommon.TEXT {
		priority := "default"
		if value.Priority != nil {
			priority = fmt.Sprint(*value.Priority)
		}
		return printVaultCredentialRows(
			helper.GetStreams().Out,
			[][5]string{{value.ID, value.CredentialType, value.Status, priority, "-"}},
			false,
		)
	}
	printer, err := cli.Format(format.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	printer.Print(value)
	return nil
}
