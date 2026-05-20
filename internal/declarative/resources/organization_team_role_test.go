package resources

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationTeamRoleValidate(t *testing.T) {
	role := OrganizationTeamRoleResource{
		Ref:            "platform-admin",
		Team:           "platform-team",
		RoleName:       "Admin",
		EntityID:       "*",
		EntityTypeName: "APIs",
		EntityRegion:   "us",
	}

	require.NoError(t, role.Validate())

	role.EntityRegion = ""
	assert.EqualError(t, role.Validate(), "entity_region is required")
}

func TestOrganizationTeamRoleRejectsKongctlMetadata(t *testing.T) {
	var role OrganizationTeamRoleResource
	err := json.Unmarshal([]byte(`{
		"ref": "platform-admin",
		"team": "platform-team",
		"role_name": "Admin",
		"entity_id": "*",
		"entity_type_name": "APIs",
		"entity_region": "us",
		"kongctl": {"namespace": "team-a"}
	}`), &role)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "kongctl metadata not supported")
}
