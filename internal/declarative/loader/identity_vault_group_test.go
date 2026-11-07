package loader

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestVaultGroupingPreservesEnablementAndScopesCollectionsPerDirectory(t *testing.T) {
	for _, tc := range []struct {
		name, fields    string
		enabled         *bool
		keys, providers bool
	}{
		{name: "omitted"},
		{name: "empty group", fields: `,"vault":{}`},
		{name: "enable only", fields: `,"vault":{"enabled":true}`, enabled: new(true)},
		{name: "disable only", fields: `,"vault":{"enabled":false}`, enabled: new(false)},
		{name: "empty keys", fields: `,"vault":{"keys":[]}`, keys: true},
		{name: "empty providers", fields: `,"vault":{"providers":[]}`, providers: true},
		{name: "both collections", fields: `,"vault":{"keys":[],"providers":[]}`, keys: true, providers: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`{"identity":{"directories":[
				{"ref":"first","name":"first"%s},
				{"ref":"second","name":"second","vault":{"providers":[]}}
			]}}`, tc.fields)
			rs, err := New().LoadFile(writeLoaderTestFile(t, input))
			require.NoError(t, err)
			require.Equal(t, tc.enabled, rs.Identity.Directories[0].VaultEnabled)
			for _, check := range []struct {
				ref  string
				kind resources.ResourceType
				want bool
			}{
				{"first", resources.ResourceTypeIdentityVaultKey, tc.keys},
				{"first", resources.ResourceTypeIdentityVaultProvider, tc.providers},
				{"second", resources.ResourceTypeIdentityVaultKey, false},
				{"second", resources.ResourceTypeIdentityVaultProvider, true},
			} {
				require.Equal(t, check.want, rs.SyncScope.ChildInScope(
					resources.ResourceTypeIdentityDirectory, check.ref, check.kind,
				))
			}
			data, err := json.Marshal(rs.Identity.Directories[0])
			require.NoError(t, err)
			require.NotContains(t, string(data), "vault_enabled")
			var roundTrip resources.IdentityDirectoryResource
			require.NoError(t, json.Unmarshal(data, &roundTrip))
			require.Equal(t, tc.enabled, roundTrip.VaultEnabled)
		})
	}
}

func TestVaultGroupingRejectsInvalidAndLegacyShapes(t *testing.T) {
	for _, fields := range []string{
		`"vault":null`, `"vault":[]`, `"vault":false`,
		`"vault":{"keys":null}`, `"vault":{"providers":null}`,
		`"vault":{"kongctl":{"namespace":"different-owner"}}`,
		`"vault":{"ref":"independent-vault"}`,
		`"vault_enabled":true`, `"vault_keys":[]`, `"vault_providers":[]`,
		`"vault":{"enabled":true},"vault_enabled":false`,
	} {
		t.Run(fields, func(t *testing.T) {
			input := `{"identity":{"directories":[{"ref":"people","name":"default",` + fields + `}]}}`
			_, err := New().LoadFile(writeLoaderTestFile(t, input))
			require.Error(t, err)
		})
	}
	for _, fields := range []string{
		`"vault":null`, `"vault":{"enabled":true}`, `"vault_keys":[]`,
		`"vault_providers":[]`, `"vault_provider_credentials":[]`,
	} {
		_, err := New().LoadFile(writeLoaderTestFile(t, `{"identity":{`+fields+`}}`))
		require.Error(t, err)
	}
}
