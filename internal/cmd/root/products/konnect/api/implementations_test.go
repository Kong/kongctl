package api

import (
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterImplementations(t *testing.T) {
	controlPlaneImplementation := newControlPlaneImplementation()
	serviceImplementation := kkComps.CreateAPIImplementationListItemAPIImplementationListItemGatewayServiceEntity(
		kkComps.APIImplementationListItemGatewayServiceEntity{
			ID: "service-implementation-id",
			Service: &kkComps.APIImplementationService{
				ID: "service-id",
			},
		},
	)
	implementations := []kkComps.APIImplementationListItem{controlPlaneImplementation, serviceImplementation}

	tests := map[string]string{
		"control plane implementation ID": "CONTROL-PLANE-IMPLEMENTATION-ID",
		"service implementation ID":       "SERVICE-IMPLEMENTATION-ID",
		"service ID":                      "SERVICE-ID",
	}
	for name, identifier := range tests {
		t.Run(name, func(t *testing.T) {
			matches := filterImplementations(implementations, identifier)
			require.Len(t, matches, 1)
		})
	}

	assert.Empty(t, filterImplementations(implementations, "missing"))
}

func TestControlPlaneImplementationFormatting(t *testing.T) {
	implementation := newControlPlaneImplementation()

	record := implementationToRecord(implementation)
	assert.Equal(t, "control-plane-implementation-id", record.ImplementationID)
	assert.Equal(t, "n/a", record.ServiceID)
	assert.Equal(t, "control-plane-id", record.ControlPlaneID)

	detail := implementationDetailView(&implementation)
	assert.Contains(t, detail, "id: control-plane-implementation-id")
	assert.Contains(t, detail, "api_id: api-id")
	assert.Contains(t, detail, "control_plane_id: control-plane-id")
	assert.Contains(t, detail, "service_id: n/a")
}

func newControlPlaneImplementation() kkComps.APIImplementationListItem {
	createdAt := time.Date(2026, time.August, 29, 12, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	return kkComps.CreateAPIImplementationListItemAPIImplementationListItemControlPlaneEntity(
		kkComps.APIImplementationListItemControlPlaneEntity{
			ID:        "control-plane-implementation-id",
			APIID:     "api-id",
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			ControlPlane: kkComps.APIImplementationControlPlane{
				ID: "control-plane-id",
			},
		},
	)
}
