package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventGatewayChildLoadRegistrations(t *testing.T) {
	want := []ResourceType{
		ResourceTypeEventGatewayStaticKey,
		ResourceTypeEventGatewayTLSTrustBundle,
		ResourceTypeEventGatewaySchemaRegistry,
		ResourceTypeEventGatewayBackendCluster,
		ResourceTypeEventGatewayListener,
		ResourceTypeEventGatewayDataPlaneCertificate,
	}

	got := make([]ResourceType, 0, len(childExtractors[ResourceTypeEventGatewayControlPlane]))
	for _, registration := range childExtractors[ResourceTypeEventGatewayControlPlane] {
		got = append(got, registration.kind)
	}

	require.Equal(t, want, got)
}
