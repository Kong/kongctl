package secrets

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestReviewedSecretCatalog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		resourceType resources.ResourceType
		path         string
		update       bool
	}{
		{resources.ResourceTypePortalIdentityProvider, "/config/client_secret", true},
		{resources.ResourceTypeDCRProvider, "/dcr_config/initial_client_secret", true},
		{resources.ResourceTypeDCRProvider, "/dcr_config/dcr_token", true},
		{resources.ResourceTypeDCRProvider, "/dcr_config/api_key", true},
		{resources.ResourceTypeAIGatewayProvider, "/config/auth/headers/0/value", true},
		{resources.ResourceTypeAIGatewayProvider, "/config/aws/secret_access_key", true},
		{resources.ResourceTypeAIGatewayProvider, "/config/gcp/service_account_json", true},
		{resources.ResourceTypeAIGatewayIdentityProvider, "/config/client_secret/0", true},
		{resources.ResourceTypeAIGatewayVault, "/config/auth/api_key", true},
		{resources.ResourceTypeAIGatewayVault, "/config/auth/token", true},
		{resources.ResourceTypeAIGatewayVault, "/config/auth/key", true},
		{resources.ResourceTypeAIGatewayVault, "/config/auth/client_secret", true},
		{resources.ResourceTypeAIGatewayVault, "/config/auth/secret_access_key", true},
		{resources.ResourceTypeAIGatewayVault, "/config/auth/secret_id", true},
		{resources.ResourceTypeEventGatewaySchemaRegistry, "/config/authentication/password", true},
		{resources.ResourceTypeAIGatewayConsumerCredential, "/api_key", false},
	}

	for _, tt := range tests {
		capability, ok := Match(tt.resourceType, tt.path)
		require.True(t, ok, "%s %s", tt.resourceType, tt.path)
		require.True(t, capability.Create)
		require.Equal(t, tt.update, capability.Update)
	}

	_, ok := Match(resources.ResourceTypeAPI, "/name")
	require.False(t, ok)
	_, ok = Match(resources.ResourceTypeAIGatewayProvider, "/config/auth/headers/0/name")
	require.False(t, ok)
}
