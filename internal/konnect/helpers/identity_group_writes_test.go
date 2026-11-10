package helpers_test

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestGroupManagementUsesDocumentedNullTypeWithoutInventingProvenance(t *testing.T) {
	group := helpers.IdentityGroup{Name: "Example", ProvisionedBy: new(kkComps.ProvisionedByKonnectUser)}
	request, err := helpers.IdentityGroupReplacement(group)
	require.NoError(t, err)
	require.Equal(t, new(kkComps.ReplaceGroupBodyTypeGroup), request.Type)
	require.Nil(t, group.Type, "read presence remains unchanged")
	group.ProvisionedBy = nil
	require.Error(t, helpers.ValidateIdentityGroupManagement(group))
	group.ProvisionedBy = new(kkComps.ProvisionedByKonnectUser)
	group.Type = new(kkComps.KongGroupTypeTeam)
	require.ErrorContains(t, helpers.ValidateIdentityGroupManagement(group), "Q32")
}
