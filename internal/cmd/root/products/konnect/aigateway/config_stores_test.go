package aigateway

import (
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConfigStorePresentation(t *testing.T) {
	displayName := "Support-Store"
	updatedAt := time.Date(2026, time.July, 24, 12, 30, 0, 0, time.UTC)
	store := kkComps.AIGatewayConfigStore{
		ID:          "11111111-1111-1111-1111-111111111111",
		Name:        "support-store",
		DisplayName: &displayName,
		CreatedAt:   updatedAt.Add(-time.Hour),
		UpdatedAt:   updatedAt,
	}

	record := aiGatewayConfigStoreToRecord(store)
	require.Equal(t, "1111…", record.ID)
	require.Equal(t, "support-store", record.Name)
	require.Equal(t, displayName, record.DisplayName)
	require.Contains(t, aiGatewayConfigStoreDetailView(store), "display_name: Support-Store")
	require.Equal(t, &store, findAIGatewayConfigStoreByNameOrID([]kkComps.AIGatewayConfigStore{store}, "support-store"))
	require.Equal(t, &store, findAIGatewayConfigStoreByNameOrID([]kkComps.AIGatewayConfigStore{store}, store.ID))
}
