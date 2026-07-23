package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAdjustControlPlaneAPIImplementationDeleteDependencies(t *testing.T) {
	t.Parallel()

	t.Run("API delete removes declared implementation relationship", func(t *testing.T) {
		t.Parallel()

		const controlPlaneID = "3285a0db-a0e6-4c18-8620-c9753c6b96ad"
		changes := []PlannedChange{
			{
				ID:           "control-plane-delete",
				ResourceType: ResourceTypeControlPlane,
				ResourceRef:  "control-plane",
				ResourceID:   controlPlaneID,
				Action:       ActionDelete,
			},
			{
				ID:           "api-delete",
				ResourceType: ResourceTypeAPI,
				ResourceRef:  "api",
				Action:       ActionDelete,
			},
		}
		rs := &resources.ResourceSet{
			APIImplementations: []resources.APIImplementationResource{{
				Ref: "implementation",
				API: "api",
				APIImplementation: kkComps.APIImplementation{
					Type: kkComps.APIImplementationTypeServiceReference,
					ServiceReference: &kkComps.ServiceReference{
						Service: &kkComps.APIImplementationService{
							ID:             "service-id",
							ControlPlaneID: controlPlaneID,
						},
					},
				},
			}},
		}

		adjustControlPlaneAPIImplementationDeleteDependencies(changes, rs)
		require.Equal(t, []string{"api-delete"}, changes[0].DependsOn)
	})

	t.Run("implementation delete precedes control plane delete", func(t *testing.T) {
		t.Parallel()

		const controlPlaneID = "3285a0db-a0e6-4c18-8620-c9753c6b96ad"
		changes := []PlannedChange{
			{
				ID:           "implementation-delete",
				ResourceType: ResourceTypeAPIImplementation,
				Action:       ActionDelete,
				Fields: map[string]any{
					FieldService: map[string]any{
						FieldControlPlaneID: controlPlaneID,
					},
				},
			},
			{
				ID:           "control-plane-delete",
				ResourceType: ResourceTypeControlPlane,
				ResourceRef:  "control-plane",
				ResourceID:   controlPlaneID,
				Action:       ActionDelete,
			},
		}

		adjustControlPlaneAPIImplementationDeleteDependencies(changes, nil)
		require.Equal(t, []string{"implementation-delete"}, changes[1].DependsOn)
	})
}
