package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIGatewayChildLoadRegistrations(t *testing.T) {
	want := []ResourceType{
		ResourceTypeAIGatewayProvider,
		ResourceTypeAIGatewayAuthStrategy,
		ResourceTypeAIGatewayPolicy,
		ResourceTypeAIGatewayAgent,
		ResourceTypeAIGatewayConsumer,
		ResourceTypeAIGatewayConsumerCredential,
		ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayModel,
		ResourceTypeAIGatewayMCPServer,
		ResourceTypeAIGatewayConfigStore,
		ResourceTypeAIGatewayConfigStoreSecret,
		ResourceTypeAIGatewayVault,
		ResourceTypeAIGatewayDataPlaneCertificate,
		ResourceTypeAIGatewayCertificate,
		ResourceTypeAIGatewayCACertificate,
		ResourceTypeAIGatewaySNI,
	}

	got := make([]ResourceType, 0, len(childValidators[ResourceTypeAIGateway]))
	for _, registration := range childValidators[ResourceTypeAIGateway] {
		got = append(got, registration.kind)
	}

	// Pins both the registered set and the diagnostic order.
	require.Equal(t, want, got)
}
