package resources

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
)

func TestRoleEntityResourceType(t *testing.T) {
	tests := []struct {
		name           string
		entityTypeName string
		expected       ResourceType
		expectedOK     bool
	}{
		{
			name:           "apis",
			entityTypeName: "APIs",
			expected:       ResourceTypeAPI,
			expectedOK:     true,
		},
		{
			name:           "services",
			entityTypeName: "Services",
			expected:       ResourceTypeAPI,
			expectedOK:     true,
		},
		{
			name:           "portals",
			entityTypeName: "Portals",
			expected:       ResourceTypePortal,
			expectedOK:     true,
		},
		{
			name:           "control planes",
			entityTypeName: "Control Planes",
			expected:       ResourceTypeControlPlane,
			expectedOK:     true,
		},
		{
			name:           "unsupported",
			entityTypeName: "Networks",
			expectedOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, ok := RoleEntityResourceType(tt.entityTypeName)
			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestOrganizationRoleDependenciesUseEntityTypeName(t *testing.T) {
	portalEntityID := tags.RefPlaceholderPrefix + "developer-portal#id"
	controlPlaneEntityID := tags.RefPlaceholderPrefix + "runtime-cp#id"

	assert.Equal(t, []ResourceRef{
		{Kind: ResourceTypeOrganizationTeam, Ref: "platform-team"},
		{Kind: ResourceTypePortal, Ref: "developer-portal"},
	}, OrganizationTeamRoleResource{
		Team:           "platform-team",
		EntityID:       portalEntityID,
		EntityTypeName: "Portals",
	}.GetDependencies())

	assert.Equal(t, []ResourceRef{
		{Kind: ResourceTypePortal, Ref: "developer-portal"},
	}, OrganizationUserRoleResource{
		EntityID:       portalEntityID,
		EntityTypeName: "Portals",
	}.GetDependencies())

	assert.Equal(t, []ResourceRef{
		{Kind: ResourceTypeControlPlane, Ref: "runtime-cp"},
	}, OrganizationSystemAccountRoleResource{
		EntityID:       controlPlaneEntityID,
		EntityTypeName: "Control Planes",
	}.GetDependencies())
}
