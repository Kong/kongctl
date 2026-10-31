package identity

import (
	"bytes"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/segmentio/cli"
	"github.com/stretchr/testify/require"
)

func TestTrustedIDPReadCommandsRequireExplicitScopes(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		for _, principals := range []bool{false, true} {
			name := "trusted-idps"
			if principals {
				name = "trusted-idp-principals"
			}
			t.Run(string(verb)+"/"+name, func(t *testing.T) {
				root, err := NewIdentityCmd(verb, nil, nil)
				require.NoError(t, err)
				command, _, err := root.Find([]string{name})
				require.NoError(t, err)
				require.NotNil(t, command.RunE)
				require.ErrorContains(t, command.Args(command, nil), "directory")
				require.NoError(t, command.Flags().Set("directory-name", "default"))
				if principals {
					require.ErrorContains(t, command.Args(command, nil), "exactly one")
				} else {
					require.NoError(t, command.Args(command, nil))
					require.NoError(t, command.Args(command, []string{id}))
					require.ErrorContains(t, command.Args(command, []string{"display name"}), "must be a UUID")
				}
				require.NoError(t, command.Flags().Set("issuer-url", "https://issuer.example.test"))
				require.NoError(t, command.Args(command, nil))
				require.Error(t, command.Args(command, []string{id}))
				require.Error(t, command.Args(command, []string{"one", "two"}))
				if principals {
					require.NoError(t, command.Flags().Set("trusted-idp-id", id))
					require.ErrorContains(t, command.Args(command, nil), "exactly one")
				}
				require.NoError(t, command.Flags().Set("issuer-url", ""))
				require.ErrorContains(t, command.Args(command, nil), "must not be empty")
			})
		}
	}
}

func TestTrustedIDPPrincipalCommandsValidateUUIDScope(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	command := newTrustedIDPReadCommand(verbs.Get, true, nil, nil)
	require.NoError(t, command.Flags().Set("directory-id", id))
	require.NoError(t, command.Flags().Set("trusted-idp-id", "invalid"))
	require.ErrorContains(t, command.Args(command, nil), "trusted-idp-id must be a UUID")
	require.NoError(t, command.Flags().Set("trusted-idp-id", id))
	require.NoError(t, command.Args(command, nil))
	require.NoError(t, command.Flags().Set("directory-name", "default"))
	require.ErrorContains(t, command.Args(command, nil), "exactly one")
}

func TestTrustedIDPTextProjectionAndPublicFormats(t *testing.T) {
	value := helpers.IdentityTrustedIDP{
		ID: "id", IssuerURL: "https://issuer.example.test",
		JitProvisioningEnabled: new(false), VaultAccessEnabled: new(true),
	}
	record := trustedIDPDisplay(value)
	require.Equal(t, "false", record.JITProvisioningEnabled)
	require.Equal(t, "true", record.VaultAccessEnabled)
	var out bytes.Buffer
	require.NoError(t, printTrustedIDPRows(&out, []helpers.IdentityTrustedIDP{value}))
	require.Contains(t, out.String(), value.IssuerURL)
	require.Contains(t, out.String(), "ISSUER_URL")
	require.Contains(t, out.String(), "false")
	require.NotContains(t, out.String(), "{")
	for _, format := range []string{"json", "yaml"} {
		out.Reset()
		printer, err := cli.Format(format, &out)
		require.NoError(t, err)
		printer.Print(value)
		printer.Flush()
		require.Contains(t, out.String(), "issuer_url")
		require.Contains(t, out.String(), "jit_provisioning_enabled")
		require.Contains(t, out.String(), "false")
	}
	unknown := trustedIDPDisplay(helpers.IdentityTrustedIDP{})
	require.Equal(t, "unknown", unknown.JITProvisioningEnabled)
	require.Equal(t, "unknown", unknown.VaultAccessEnabled)
}
