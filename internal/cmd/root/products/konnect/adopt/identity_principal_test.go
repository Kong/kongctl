package adopt

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestAdoptPrincipalPreservesFieldsAndRejectsConflicts(t *testing.T) {
	directoryID := "7a5e1b58-48a4-4586-b02d-0d91eccca152"
	id := "1248f559-063b-4f10-9098-9e6553b21d9b"
	current := kkComps.KongPrincipal{
		ID:          id,
		DisplayName: new("Unowned"),
		Description: "Keep",
		Type:        new(kkComps.KongPrincipalTypeAgent),
		Labels:      map[string]string{"owner": "team"},
	}
	api := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directoryID: {current}}}
	result, err := adoptIdentityPrincipal(t.Context(), api, directoryID, id, "agent", "identity-team", false)
	require.NoError(t, err)
	require.Equal(t, "agent", result.Name)
	require.Len(t, api.Writes, 1)
	require.Equal(t, "Keep", *api.Writes[0].Description)
	require.Equal(t, current.Type, api.Writes[0].Type)
	require.Equal(t, current.Labels, api.Writes[0].Labels)
	require.Len(t, api.Writes[0].ManagedBy, 5)
	require.Nil(t, api.Writes[0].Active)
	_, err = adoptIdentityPrincipal(t.Context(), api, directoryID, id, "agent", "identity-team", false)
	require.ErrorContains(t, err, "overwrite-namespace")
	duplicate := current
	duplicate.ID = "2248f559-063b-4f10-9098-9e6553b21d9b"
	duplicate.ManagedBy = resources.BuildPrincipalManagedBy("identity-team", "agent", false, "old")
	api.Directories[directoryID] = append(api.Directories[directoryID], duplicate)
	_, err = adoptIdentityPrincipal(t.Context(), api, directoryID, id, "agent", "identity-team", true)
	require.ErrorContains(t, err, "another principal")
	require.Len(t, api.Writes, 1)
}
