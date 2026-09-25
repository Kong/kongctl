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
					Type: kkComps.APIImplementationTypeServiceReferenceInput,
					ServiceReferenceInput: &kkComps.ServiceReferenceInput{
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

	t.Run("control plane implementation delete precedes control plane delete", func(t *testing.T) {
		t.Parallel()

		const controlPlaneID = "3285a0db-a0e6-4c18-8620-c9753c6b96ad"
		changes := []PlannedChange{
			{
				ID:           "implementation-delete",
				ResourceType: ResourceTypeAPIImplementation,
				Action:       ActionDelete,
				Fields: map[string]any{
					FieldControlPlane: map[string]any{FieldControlPlaneID: controlPlaneID},
				},
			},
			{
				ID:           "control-plane-delete",
				ResourceType: ResourceTypeControlPlane,
				ResourceID:   controlPlaneID,
				Action:       ActionDelete,
			},
		}

		adjustControlPlaneAPIImplementationDeleteDependencies(changes, nil)
		require.Equal(t, []string{"implementation-delete"}, changes[1].DependsOn)
	})
}

func TestControlPlaneDeleteDependenciesWithDifferentDeclarationRefs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, namespace       string
		controlPlaneReference bool
	}{
		{"service reference", DefaultNamespace, false},
		{"control plane reference", DefaultNamespace, true},
		{"unrelated namespace", "other", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changes := []PlannedChange{
				{
					ID: "cp-delete", ResourceType: ResourceTypeControlPlane, ResourceRef: "code-breakers", ResourceID: "cp-id",
					Action: ActionDelete, Namespace: tc.namespace, Fields: map[string]any{FieldName: "code-breakers"},
				},
				{
					ID: "api-delete", ResourceType: ResourceTypeAPI, ResourceRef: "code-breakers", ResourceID: "api-id",
					Action: ActionDelete, Namespace: tc.namespace, Fields: map[string]any{FieldName: "code-breakers"},
				},
			}
			implementation := kkComps.CreateAPIImplementationServiceReferenceInput(kkComps.ServiceReferenceInput{
				Service: &kkComps.APIImplementationService{
					ID:             "service-id",
					ControlPlaneID: "__REF__:code-breakers-cp#id",
				},
			})
			if tc.controlPlaneReference {
				implementation = kkComps.CreateAPIImplementationControlPlaneReference(kkComps.ControlPlaneReference{
					ControlPlane: &kkComps.APIImplementationControlPlaneInput{ID: "__REF__:code-breakers-cp#id"},
				})
			}
			rs := &resources.ResourceSet{
				ControlPlanes: []resources.ControlPlaneResource{{
					BaseResource:              resources.BaseResource{Ref: "code-breakers-cp"},
					CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: "code-breakers"},
				}},
				APIs: []resources.APIResource{{
					BaseResource:     resources.BaseResource{Ref: "code-breakers-api"},
					CreateAPIRequest: kkComps.CreateAPIRequest{Name: "code-breakers"},
				}},
				APIImplementations: []resources.APIImplementationResource{{
					Ref: "implementation", API: "code-breakers-api", APIImplementation: implementation,
				}},
			}
			adjustControlPlaneAPIImplementationDeleteDependencies(changes, rs)
			if tc.namespace != DefaultNamespace {
				require.Empty(t, changes[0].DependsOn)
				return
			}
			require.Equal(t, []string{"api-delete"}, changes[0].DependsOn)
			result, err := NewDependencyResolver().ResolveDependenciesWithGroups(changes)
			require.NoError(t, err)
			require.Equal(t, [][]string{{"api-delete"}, {"cp-delete"}}, result.ExecutionGroups)
		})
	}
}
