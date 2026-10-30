package aigateway

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayDatastoreReadCommands(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		root, err := NewAIGatewayCmd(verb, nil, nil)
		require.NoError(t, err)
		child, _, err := root.Find([]string{"datastores"})
		require.NoError(t, err)
		require.Equal(t, "datastores", child.Name())
		require.NotNil(t, child.Flags().Lookup("gateway-id"))
		require.NotNil(t, child.Flags().Lookup("datastore-id"))
	}
}

func TestAIGatewayDatastoreSummarySelectionAndDetail(t *testing.T) {
	values := []kkComps.AIGatewayDatastoreSummary{
		{ID: "store-id", Name: "store", Type: kkComps.AIGatewayDatastoreSummaryTypeTokenVault},
	}
	for _, identifier := range []string{"store", "store-id"} {
		item := findAIGatewayDatastoreByNameOrID(values, identifier)
		require.NotNil(t, item)
		require.Equal(t, "store-id", aiGatewayDatastoreToRecord(*item).ID)
		require.Contains(t, aiGatewayDatastoreDetailView(*item), "type: token-vault")
	}
	require.Nil(t, findAIGatewayDatastoreByNameOrID(values, "missing"))
}
