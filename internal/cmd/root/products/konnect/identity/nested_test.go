package identity

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestNestedIdentityReadCommands(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		for _, tc := range []struct {
			path, flags []string
		}{
			{path: []string{"principal", "identities"}, flags: []string{"--principal-id", id}},
			{path: []string{"principals", "api-keys"}, flags: []string{"--principal-id", id}},
			{path: []string{"principal", "basic-auths"}, flags: []string{"--principal-id", id}},
			{
				path:  []string{"principal", "basic-auth", "passwords"},
				flags: []string{"--principal-id", id, "--username", "login"},
			},
			{path: []string{"principal", "groups"}, flags: []string{"--principal-id", id}},
			{path: []string{"principal", "vault-credentials"}, flags: []string{"--principal-id", id}},
			{path: []string{"group", "api-keys"}, flags: []string{"--group-id", id}},
			{
				path:  []string{"trusted-idp", "principals"},
				flags: []string{"--issuer-url", "https://issuer.example.test"},
			},
			{path: []string{"trusted-idps", "principal"}, flags: []string{"--trusted-idp-id", id}},
			{path: []string{"vault", "keys"}},
			{path: []string{"vault", "key"}, flags: []string{"--kid", "public-key-fingerprint"}},
			{path: []string{"vault", "providers"}},
			{path: []string{"vault", "provider"}, flags: []string{"--provider-name", "support-vendor"}},
			{path: []string{"vault", "provider", "credentials"}, flags: []string{"--provider-name", "support-vendor"}},
			{path: []string{"vault", "providers", "credential"}, flags: []string{id, "--provider-id", id}},
		} {
			t.Run(string(verb)+"/"+strings.Join(tc.path, "/"), func(t *testing.T) {
				runNestedIdentityCommand(t, verb, tc.path, tc.flags)
			})
		}
	}
}

func TestNestedTrustedIDPPrincipalsPreserveSelectorsAndFlatForms(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		root, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		parent, _, err := root.Find([]string{"trusted-idp"})
		require.NoError(t, err)
		command, remaining, err := root.Find([]string{"trusted-idps", "principals"})
		require.NoError(t, err)
		require.Empty(t, remaining)
		require.Equal(t, parent, command.Parent())
		flat, _, err := root.Find([]string{"trusted-idp-principals"})
		require.NoError(t, err)
		require.NotSame(t, flat, command)
		require.Equal(t, root, flat.Parent())
		require.NoError(t, parent.Flags().Set("issuer-url", "https://parent.example.test"))
		require.ErrorContains(t, command.Args(command, nil), "exactly one")
		require.NoError(t, command.Flags().Set("trusted-idp-id", "invalid"))
		require.ErrorContains(t, command.Args(command, nil), "must be a UUID")
		require.NoError(t, command.Flags().Set("trusted-idp-id", id))
		require.NoError(t, command.Args(command, nil))
		require.Error(t, command.Args(command, []string{id}))
		require.NoError(t, command.Flags().Set("issuer-url", "https://child.example.test"))
		require.ErrorContains(t, command.Args(command, nil), "exactly one")
		value, err := flat.Flags().GetString("issuer-url")
		require.NoError(t, err)
		require.Empty(t, value)
		value, err = flat.Flags().GetString("trusted-idp-id")
		require.NoError(t, err)
		require.Empty(t, value)
	}
}

func TestNestedVaultCommandsPreserveSelectorsAndFlatForms(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		root, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		key, _, err := root.Find([]string{"vault", "keys"})
		require.NoError(t, err)
		flat, _, err := root.Find([]string{"vault-keys"})
		require.NoError(t, err)
		require.NotSame(t, flat, key)
		require.Equal(t, root, flat.Parent())
		require.NoError(t, key.Args(key, []string{id}))
		require.NoError(t, key.Flags().Set("kid", "fingerprint"))
		require.ErrorContains(t, key.Args(key, []string{id}), "not both")
		value, err := flat.Flags().GetString("kid")
		require.NoError(t, err)
		require.Empty(t, value)
		provider, _, err := root.Find([]string{"vault", "providers"})
		require.NoError(t, err)
		require.NoError(t, provider.Args(provider, []string{"support-vendor"}))
		require.NoError(t, provider.Flags().Set("provider-name", "support-vendor"))
		require.ErrorContains(t, provider.Args(provider, []string{id}), "not both")
		credential, remaining, err := root.Find([]string{"vault", "providers", "credentials"})
		require.NoError(t, err)
		require.Empty(t, remaining)
		require.Equal(t, provider, credential.Parent())
		flatCredential, _, err := root.Find([]string{"vault-provider-credentials"})
		require.NoError(t, err)
		require.NotSame(t, flatCredential, credential)
		require.ErrorContains(t, credential.Args(credential, nil), "provider")
		require.NoError(t, credential.Flags().Set("provider-name", "support-vendor"))
		require.NoError(t, credential.Args(credential, []string{id}))
		require.ErrorContains(t, credential.Args(credential, []string{"invalid"}), "UUID")
		value, err = flatCredential.Flags().GetString("provider-name")
		require.NoError(t, err)
		require.Empty(t, value)
		vault := key.Parent()
		require.Equal(t, "vault", vault.Name())
		require.Empty(t, vault.Aliases)
		require.Error(t, vault.RunE(vault, nil))
	}
}

func TestNestedVaultCredentialMutations(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, verb := range []verbs.VerbValue{verbs.Create, verbs.Delete} {
		for _, noun := range []string{"credential", "credentials"} {
			t.Run(string(verb)+"/"+noun, func(t *testing.T) {
				flags := []string{"--provider-name", "support-vendor"}
				if verb == verbs.Create {
					flags = append(flags, "--secret-output-file", "private-capture.json")
				} else {
					flags = append(flags, id)
				}
				runNestedIdentityCommand(t, verb, []string{"vault", "provider", noun}, flags)
				root, err := NewIdentityCmd(verb, nil, nil)
				require.NoError(t, err)
				command, _, err := root.Find([]string{"vault", "provider", noun})
				require.NoError(t, err)
				require.ErrorContains(t, command.Args(command, nil), "provider")
				require.NoError(t, command.Flags().Set("provider-id", id))
				flat, _, err := root.Find([]string{"vault-provider-credentials"})
				require.NoError(t, err)
				require.NotSame(t, flat, command)
				if verb == verbs.Create {
					require.NoError(t, command.Args(command, nil))
					require.ErrorContains(t, command.ValidateRequiredFlags(), "secret-output-file")
				} else {
					require.Error(t, command.Args(command, nil))
					require.ErrorContains(t, command.Args(command, []string{"invalid"}), "UUID")
					require.NoError(t, command.Args(command, []string{id}))
				}
			})
		}
	}
	runNestedIdentityCommand(t, verbs.Create, []string{"vault", "provider", "enrollment"},
		[]string{id, "--provider-name", "support-vendor", "--secret-output-file", "private-capture.json"})
}

func TestNestedPrincipalCredentialMutationsAndAdoption(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, verb := range []verbs.VerbValue{verbs.Create, verbs.Patch, verbs.Delete} {
		for _, tc := range []struct {
			path, flags []string
		}{
			{path: []string{"principal", "api-keys"}, flags: []string{"--principal-id", id}},
			{path: []string{"groups", "api-key"}, flags: []string{"--group-id", id}},
			{
				path:  []string{"principal", "basic-auths", "password"},
				flags: []string{"--principal-id", id, "--username", "login"},
			},
		} {
			t.Run(string(verb)+"/"+strings.Join(tc.path, "/"), func(t *testing.T) {
				flags := tc.flags
				if verb == verbs.Create {
					flags = append(flags, "--secret-output-file", "private-capture.json")
				} else {
					flags = append(flags, id)
					if verb == verbs.Patch {
						flags = append(flags, "--no-expiration")
					}
				}
				runNestedIdentityCommand(t, verb, tc.path, flags)
			})
		}
	}
	for _, tc := range []struct {
		path, flags []string
	}{
		{path: []string{"principal", "identities"}, flags: []string{id, "--principal-id", id, "--ref", "external-login"}},
		{path: []string{"principal", "basic-auth"}, flags: []string{"--principal-id", id, "--username", "login"}},
	} {
		t.Run("adopt/"+strings.Join(tc.path, "/"), func(t *testing.T) {
			runNestedIdentityCommand(t, verbs.Adopt, tc.path, tc.flags)
		})
	}
}

func runNestedIdentityCommand(t *testing.T, verb verbs.VerbValue, path, flags []string) {
	t.Helper()
	preRuns := 0
	identity, err := NewIdentityCmd(verb, nil, func(*cobra.Command, []string) error {
		preRuns++
		return nil
	})
	require.NoError(t, err)
	root := &cobra.Command{Use: string(verb), SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(identity)
	fullPath := append([]string{"id"}, path...)
	leaf, remaining, err := root.Find(fullPath)
	require.NoError(t, err)
	require.Empty(t, remaining)
	require.NotNil(t, leaf.RunE)
	called := false
	leaf.RunE = func(cmd *cobra.Command, _ []string) error {
		called = true
		id, name, err := common.IdentityDirectorySelector(cmd)
		require.NoError(t, err)
		require.Empty(t, id)
		require.Equal(t, "default", name)
		return nil
	}
	root.SetArgs(append(fullPath, flags...))
	require.NoError(t, root.Execute())
	require.True(t, called)
	require.Equal(t, 1, preRuns)
}

func TestNestedPrincipalCommandsPreserveValidationAndFlatForms(t *testing.T) {
	root, err := NewIdentityCmd(verbs.Get, nil, nil)
	require.NoError(t, err)
	nested, _, err := root.Find([]string{"principal", "api-keys"})
	require.NoError(t, err)
	flat, _, err := root.Find([]string{"principal-api-keys"})
	require.NoError(t, err)
	require.NotSame(t, flat, nested)
	require.Equal(t, root, flat.Parent())
	require.ErrorContains(t, nested.Args(nested, nil), "principal-id")
	require.NoError(t, nested.Flags().Set("principal-id", "731559d3-39bd-48c7-b33c-eb192847eb20"))
	require.NoError(t, nested.Args(nested, nil))
	value, err := flat.Flags().GetString("principal-id")
	require.NoError(t, err)
	require.Empty(t, value)
	identity, rest, err := root.Find([]string{"principal", "identity", "api-keys"})
	require.NoError(t, err)
	require.Equal(t, []string{"api-keys"}, rest)
	require.Error(t, identity.Args(identity, rest))
	password, _, err := root.Find([]string{"principal", "basic-auth", "passwords"})
	require.NoError(t, err)
	require.NoError(t, password.Flags().Set("principal-id", "731559d3-39bd-48c7-b33c-eb192847eb20"))
	require.ErrorContains(t, password.Args(password, nil), "exactly one")
}

func TestNestedCredentialCommandsRequireCaptureAndMutationIntent(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, path := range [][]string{
		{"principal", "api-key"},
		{"principal", "basic-auth", "password"},
	} {
		for _, verb := range []verbs.VerbValue{verbs.Create, verbs.Patch} {
			t.Run(string(verb)+"/"+strings.Join(path, "/"), func(t *testing.T) {
				root, err := NewIdentityCmd(verb, nil, nil)
				require.NoError(t, err)
				command, _, err := root.Find(path)
				require.NoError(t, err)
				require.NoError(t, command.Flags().Set("principal-id", id))
				if path[len(path)-1] == "password" {
					require.NoError(t, command.Flags().Set("username", "login"))
				}
				if verb == verbs.Create {
					require.NoError(t, command.Args(command, nil))
					require.ErrorContains(t, command.ValidateRequiredFlags(), "secret-output-file")
				} else {
					require.ErrorContains(t, command.Args(command, []string{id}), "provide --expires-at")
				}
			})
		}
	}
}
