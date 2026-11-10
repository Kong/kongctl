package state

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestIdentityDirectoryReportsIgnoredVaultSetting(t *testing.T) {
	api := &fixture.DirectoryAPI{Current: fixture.Directory("people", "identity-team", false)}
	client := NewClient(ClientConfig{IdentityDirectoriesAPI: api})
	err := client.ReplaceIdentityDirectory(t.Context(), api.Current.ID,
		kkComps.ReplaceDirectoryBody{Name: "people", VaultEnabled: new(true), ManagedBy: api.Current.ManagedBy})
	require.ErrorContains(t, err, "did not retain vault_enabled=true")
	require.True(t, *api.Replaced.VaultEnabled, "the SDK request must carry the requested true value")
}
