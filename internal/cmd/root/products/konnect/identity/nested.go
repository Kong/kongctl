package identity

import (
	"strings"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
)

// Construct separate commands so the flat forms retain their own flags and parent paths.
func addNestedIdentityCommands(root *cobra.Command, verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) error {
	//exhaustive:ignore
	switch verb {
	case verbs.Get, verbs.List:
		trustedIDP := identityResourceGroup(root, "trusted-idp", "trusted-idps")
		trustedIDP.AddCommand(renameNestedIdentityCommand(
			newTrustedIDPReadCommand(verb, true, addParentFlags, parentPreRun), "principal", "principals",
		))
		vault := &cobra.Command{Use: "vault", Short: "Inspect directory Token Vault keys and providers"}
		cmdpkg.ConfigureRequiresSubcommand(vault)
		provider := renameNestedIdentityCommand(newVaultProviderReadCommand(verb, false, addParentFlags, parentPreRun),
			"provider", "providers")
		providerCredential, err := newVaultCredentialReadCommand(verb, false, addParentFlags, parentPreRun)
		if err != nil {
			return err
		}
		provider.AddCommand(renameNestedIdentityCommand(providerCredential, "credential", "credentials"))
		vault.AddCommand(
			renameNestedIdentityCommand(newVaultKeyReadCommand(verb, addParentFlags, parentPreRun), "key", "keys"),
			provider,
		)
		root.AddCommand(vault)
		principal := identityResourceGroup(root, "principal", "principals")
		identity, err := newPrincipalIdentityCommand(verb, addParentFlags, parentPreRun)
		if err != nil {
			return err
		}
		principal.AddCommand(renameNestedIdentityCommand(identity, "identity", "identities"))
		basicAuth, err := newBasicAuthReadCommand(verb, false, addParentFlags, parentPreRun)
		if err != nil {
			return err
		}
		password, err := newBasicAuthReadCommand(verb, true, addParentFlags, parentPreRun)
		if err != nil {
			return err
		}
		basicAuth.AddCommand(renameNestedIdentityCommand(password, "password", "passwords"))
		principal.AddCommand(renameNestedIdentityCommand(basicAuth, "basic-auth", "basic-auths"))
		groups, err := newPrincipalGroupReadCommand(verb, addParentFlags, parentPreRun)
		if err != nil {
			return err
		}
		principal.AddCommand(renameNestedIdentityCommand(groups, "group", "groups"))
		credentials, err := newVaultCredentialReadCommand(verb, true, addParentFlags, parentPreRun)
		if err != nil {
			return err
		}
		principal.AddCommand(renameNestedIdentityCommand(credentials, "vault-credential", "vault-credentials"))
	case verbs.Adopt:
		principal := identityResourceGroup(root, "principal", "principals")
		identity, err := adopt.NewIdentityPrincipalIdentityCmd(verb, addParentFlags, parentPreRun)
		if err != nil {
			return err
		}
		basicAuth, err := adopt.NewIdentityBasicAuthCmd(verb, addParentFlags, parentPreRun)
		if err != nil {
			return err
		}
		principal.AddCommand(renameNestedIdentityCommand(identity, "identity", "identities"),
			renameNestedIdentityCommand(basicAuth, "basic-auth", "basic-auths"))
	case verbs.Create, verbs.Patch, verbs.Delete:
		if verb != verbs.Patch {
			vault := &cobra.Command{Use: "vault", Short: "Manage directory Token Vault credentials"}
			cmdpkg.ConfigureRequiresSubcommand(vault)
			provider := identityResourceGroup(vault, "provider", "providers")
			credential, err := newVaultCredentialMutationCommand(verb, false, addParentFlags, parentPreRun)
			if err != nil {
				return err
			}
			provider.AddCommand(renameNestedIdentityCommand(credential, "credential", "credentials"))
			if verb == verbs.Create {
				enrollment, err := newVaultCredentialMutationCommand(verb, true, addParentFlags, parentPreRun)
				if err != nil {
					return err
				}
				provider.AddCommand(renameNestedIdentityCommand(enrollment, "enrollment", "enrollments"))
			}
			root.AddCommand(vault)
		}
		principal := identityResourceGroup(root, "principal", "principals")
		basicAuth := identityResourceGroup(principal, "basic-auth", "basic-auths")
		password, err := newPasswordMutationCommand(verb, addParentFlags, parentPreRun)
		if err != nil {
			return err
		}
		basicAuth.AddCommand(renameNestedIdentityCommand(password, "password", "passwords"))
	default:
		return nil
	}
	if verb == verbs.Get || verb == verbs.List || verb == verbs.Create || verb == verbs.Patch || verb == verbs.Delete {
		for _, groupKeys := range []bool{false, true} {
			name, alias := "principal", "principals"
			if groupKeys {
				name, alias = "group", "groups"
			}
			parent := identityResourceGroup(root, name, alias)
			key, err := newIdentityAPIKeyCommand(verb, groupKeys, addParentFlags, parentPreRun)
			if err != nil {
				return err
			}
			parent.AddCommand(renameNestedIdentityCommand(key, "api-key", "api-keys"))
		}
	}
	return nil
}

func identityResourceGroup(parent *cobra.Command, name, alias string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	group := &cobra.Command{Use: name, Aliases: []string{alias}, Short: "Manage Identity " + name + " child resources"}
	cmdpkg.ConfigureRequiresSubcommand(group)
	parent.AddCommand(group)
	return group
}

func renameNestedIdentityCommand(command *cobra.Command, name, alias string) *cobra.Command {
	_, arguments, _ := strings.Cut(command.Use, " ")
	command.Use = name
	if arguments != "" {
		command.Use += " " + arguments
	}
	command.Aliases = []string{alias}
	return command
}
