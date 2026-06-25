package planner

import (
	"testing"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestExtractEGWControlPlaneFieldsIncludesMinRuntimeVersion(t *testing.T) {
	minRuntimeVersion := "1.2"
	fields := extractEGWControlPlaneFields(resources.EventGatewayControlPlaneResource{
		CreateGatewayRequest: components.CreateGatewayRequest{
			Name:              "event-gateway",
			MinRuntimeVersion: &minRuntimeVersion,
		},
	})

	require.Equal(t, "1.2", fields[FieldMinRuntimeVersion])
}

func TestShouldUpdateEGWControlPlaneResourceDetectsMinRuntimeVersionChange(t *testing.T) {
	minRuntimeVersion := "1.2"
	current := state.EventGatewayControlPlane{
		EventGatewayInfo: components.EventGatewayInfo{
			ID:                "event-gateway-id",
			Name:              "event-gateway",
			MinRuntimeVersion: "1.1",
		},
	}
	desired := resources.EventGatewayControlPlaneResource{
		CreateGatewayRequest: components.CreateGatewayRequest{
			Name:              "event-gateway",
			MinRuntimeVersion: &minRuntimeVersion,
		},
	}

	needsUpdate, updates, changed := (&Planner{}).shouldUpdateEGWControlPlaneResource(current, desired)

	require.True(t, needsUpdate)
	require.Equal(t, "1.2", updates[FieldMinRuntimeVersion])
	require.Equal(t, FieldChange{Old: "1.1", New: "1.2"}, changed[FieldMinRuntimeVersion])
}
