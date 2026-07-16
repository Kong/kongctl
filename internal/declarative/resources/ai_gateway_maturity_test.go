package resources

import (
	"testing"

	"github.com/kong/kongctl/internal/maturity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayResourcesAreBeta(t *testing.T) {
	resourceTypes := []ResourceType{
		ResourceTypeAIGateway,
		ResourceTypeAIGatewayProvider,
		ResourceTypeAIGatewayIdentityProvider,
		ResourceTypeAIGatewayPolicy,
		ResourceTypeAIGatewayAgent,
		ResourceTypeAIGatewayConsumer,
		ResourceTypeAIGatewayConsumerCredential,
		ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayModel,
		ResourceTypeAIGatewayMCPServer,
		ResourceTypeAIGatewayVault,
		ResourceTypeAIGatewayDataPlaneCertificate,
	}

	for _, resourceType := range resourceTypes {
		t.Run(string(resourceType), func(t *testing.T) {
			resolved, err := MaturityFor(resourceType)
			require.NoError(t, err)
			assert.Equal(t, maturity.LevelBeta, resolved.Effective.Level)
			assert.Equal(t, maturity.KindResource, resolved.Source.Kind)

			for _, operation := range Operations() {
				operationResolved, err := MaturityFor(resourceType, operation)
				require.NoError(t, err)
				assert.Equal(t, maturity.LevelBeta, operationResolved.Effective.Level, operation)
				assert.Equal(t, maturity.KindResource, operationResolved.Source.Kind, operation)
			}

			subject, err := ResolveExplainSubject(string(resourceType))
			require.NoError(t, err)
			schema := RenderExplainSchema(subject)
			require.NotNil(t, schema.XMaturity)
			assert.Equal(t, maturity.LevelBeta, schema.XMaturity.Level)
		})
	}
}
