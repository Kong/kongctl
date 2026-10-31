package identity

import (
	"bytes"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/segmentio/cli"
	"github.com/stretchr/testify/require"
)

func TestVaultKeyReadCommandScopesAndFingerprintSelection(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		for _, name := range []string{"vault-key", "vault-keys", "vault_keys"} {
			t.Run(string(verb)+"/"+name, func(t *testing.T) {
				root, err := NewIdentityCmd(verb, nil, nil)
				require.NoError(t, err)
				command, _, err := root.Find([]string{name})
				require.NoError(t, err)
				require.NotNil(t, command.RunE)
				require.ErrorContains(t, command.Args(command, nil), "directory")
				require.NoError(t, command.Flags().Set("directory-name", "default"))
				require.NoError(t, command.Args(command, nil))
				require.NoError(t, command.Args(command, []string{id}))
				require.ErrorContains(t, command.Args(command, []string{"a-name"}), "must be a UUID")
				require.Error(t, command.Args(command, []string{id, id}))
				require.NoError(t, command.Flags().Set("kid", "fingerprint"))
				require.NoError(t, command.Args(command, nil))
				require.ErrorContains(t, command.Args(command, []string{id}), "not both")
				require.NoError(t, command.Flags().Set("kid", ""))
				require.ErrorContains(t, command.Args(command, nil), "must not be empty")
			})
		}
	}
	command := newVaultKeyReadCommand(verbs.Get, nil, nil)
	require.NoError(t, command.Flags().Set("directory-id", "invalid"))
	require.ErrorContains(t, command.Args(command, nil), "UUID")
	require.NoError(t, command.Flags().Set("directory-id", id))
	require.NoError(t, command.Args(command, nil))
	require.NoError(t, command.Flags().Set("directory-name", "default"))
	require.ErrorContains(t, command.Args(command, nil), "exactly one")
}

func TestVaultKeyTextSummaryAndPublicStructuredFormats(t *testing.T) {
	value := helpers.IdentityVaultKey{
		ID: "id", DirectoryID: "directory", Name: "shared-name", Kid: "kid", Algorithm: "RSA", KeySize: 2048,
		PublicKey: "-----BEGIN PUBLIC KEY-----\npublic-only\n-----END PUBLIC KEY-----",
	}
	record := vaultKeyDisplay(value)
	require.Equal(t, value.Kid, record.Kid)
	require.Equal(t, value.KeySize, record.KeySize)
	var out bytes.Buffer
	require.NoError(t, printVaultKeyRows(&out, []helpers.IdentityVaultKey{value}))
	require.Contains(t, out.String(), "KID")
	require.Contains(t, out.String(), "shared-name")
	require.Contains(t, out.String(), "2048")
	require.NotContains(t, out.String(), "BEGIN PUBLIC KEY")
	require.NotContains(t, out.String(), "{")
	for _, format := range []string{"json", "yaml"} {
		out.Reset()
		printer, err := cli.Format(format, &out)
		require.NoError(t, err)
		printer.Print(value)
		printer.Flush()
		require.Contains(t, out.String(), "directory_id")
		require.Contains(t, out.String(), "public_key")
		require.Contains(t, out.String(), "BEGIN PUBLIC KEY")
	}
}
