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
		{resources.ResourceTypeAIGatewayProvider, "/config/auth/client_secret", true},
		{resources.ResourceTypeAIGatewayProvider, "/config/auth/secret_access_key", true},
		{resources.ResourceTypeAIGatewayProvider, "/config/auth/aws/secret_access_key", true},
		{resources.ResourceTypeAIGatewayProvider, "/config/auth/service_account_json", true},
		{resources.ResourceTypeAIGatewayAuthStrategy, "/config/client_secret/0", true},
		{resources.ResourceTypeAIGatewayVault, "/config/api_key", true},
		{resources.ResourceTypeAIGatewayVault, "/config/token", true},
		{resources.ResourceTypeAIGatewayVault, "/config/client_secret", true},
		{resources.ResourceTypeAIGatewayVault, "/config/secret_access_key", true},
		{resources.ResourceTypeAIGatewayVault, "/config/secret_id", true},
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
	_, ok = Match(resources.ResourceTypeAIGatewayProvider, "/config/foundry/headers/0/value")
	require.False(t, ok)
	_, ok = Match(resources.ResourceTypeAIGatewayVault, "/config/key")
	require.False(t, ok)
	_, ok = Match(resources.ResourceTypeAIGatewayVault, "/config/auth/token")
	require.False(t, ok)
}

func TestIsVaultReference(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"{vault://support-secrets/openai-auth-header}",
		"  {vault://support-secrets/openai-auth-header}  ",
		"${vault['my-vault']['key']}",
	} {
		require.True(t, IsVaultReference(value), value)
	}

	for _, value := range []string{
		"plaintext-secret",
		"{vault://}",
		"${vault[}",
		"${env['SECRET']}",
		"vault://support-secrets/openai-auth-header",
	} {
		require.False(t, IsVaultReference(value), value)
	}
}
