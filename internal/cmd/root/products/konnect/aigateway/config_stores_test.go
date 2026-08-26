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
	require.Equal(t, store.ID, record.ID)
	require.Equal(t, "support-store", record.Name)
	require.Equal(t, displayName, record.DisplayName)
	require.Contains(t, aiGatewayConfigStoreDetailView(store), "display_name: Support-Store")
}

func TestAIGatewayConfigStoreSecretPresentationContainsOnlyMetadata(t *testing.T) {
	updatedAt := time.Date(2026, time.August, 25, 12, 30, 0, 0, time.UTC)
	secret := kkComps.AIGatewayConfigStoreSecret{
		Key:       "openai-auth-header",
		CreatedAt: updatedAt.Add(-time.Hour),
		UpdatedAt: updatedAt,
	}

	record := aiGatewayConfigStoreSecretToRecord(secret)
	require.Equal(t, secret.Key, record.Key)
	detail := aiGatewayConfigStoreSecretDetailView(secret)
	require.Contains(t, detail, "key: openai-auth-header")
	require.Contains(t, detail, "created_at:")
	require.Contains(t, detail, "updated_at:")
	require.NotContains(t, detail, "value")
}
