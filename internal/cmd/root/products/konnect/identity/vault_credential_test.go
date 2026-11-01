package identity

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/segmentio/cli"
	"github.com/stretchr/testify/require"
)

func TestVaultCredentialReadCommandsRequireExplicitParentScopes(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		for _, principal := range []bool{false, true} {
			command, err := newVaultCredentialReadCommand(verb, principal, nil, nil)
			require.NoError(t, err)
			require.Error(t, command.Args(command, nil))
			require.NoError(t, command.Flags().Set("directory-name", "default"))
			require.Error(t, command.Args(command, nil))
			parent := "provider-id"
			if principal {
				parent = "principal-id"
			}
			require.NoError(t, command.Flags().Set(parent, id))
			require.NoError(t, command.Args(command, nil))
			require.NoError(t, command.Args(command, []string{id}))
			require.Error(t, command.Args(command, []string{"invalid"}))
			require.Error(t, command.Args(command, []string{id, id}))
			if !principal {
				require.NoError(t, command.Flags().Set("provider-name", "github-work"))
				require.Error(t, command.Args(command, nil))
				require.NoError(t, command.Flags().Set("provider-id", ""))
				require.NoError(t, command.Args(command, []string{id}))
			}
			root, err := NewIdentityCmd(verb, nil, nil)
			require.NoError(t, err)
			for _, name := range append([]string{command.Name()}, command.Aliases...) {
				found, _, err := root.Find([]string{name})
				require.NoError(t, err)
				require.Equal(t, command.Name(), found.Name())
			}
		}
	}
}

func TestVaultCredentialOutputFormatsArePublicAndVariantSpecific(t *testing.T) {
	var value helpers.IdentityVaultProviderCredential
	require.NoError(t, json.Unmarshal([]byte(`{"id":"id","credential_type":"shared","status":"pending",`+
		`"selector":null,"priority":null,"enrollment_url":"private-fixture-value",`+
		`"secret":"private-fixture-value"}`), &value))
	for _, format := range []string{"json", "yaml"} {
		var out bytes.Buffer
		printer, err := cli.Format(format, &out)
		require.NoError(t, err)
		printer.Print(value)
		printer.Flush()
		require.Contains(t, out.String(), "pending")
		require.Contains(t, out.String(), "selector")
		require.NotContains(t, out.String(), "private-fixture-value")
		require.NotContains(t, out.String(), "enrollment_url")
		require.NotContains(t, out.String(), "principal")
		require.NotContains(t, out.String(), "expires_at")
	}
	for _, principal := range []bool{false, true} {
		var out bytes.Buffer
		require.NoError(
			t,
			printVaultCredentialRows(&out, [][5]string{{"id", "shared", "pending", "default", "-"}}, principal),
		)
		require.Contains(t, out.String(), "ID")
		require.NotContains(t, out.String(), "{")
	}
}
