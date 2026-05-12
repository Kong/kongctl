package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPortalTeamGroupMappingValidateAllowsEmptyGroups(t *testing.T) {
	mapping := PortalTeamGroupMappingResource{
		Ref:    "developers-idp-groups",
		Team:   "developers",
		Groups: []string{},
	}

	require.NoError(t, mapping.Validate())
}

func TestPortalTeamGroupMappingValidateRejectsDuplicateGroups(t *testing.T) {
	mapping := PortalTeamGroupMappingResource{
		Ref:    "developers-idp-groups",
		Team:   "developers",
		Groups: []string{"Service Developer", "Service Developer"},
	}

	err := mapping.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), `duplicate group name "Service Developer"`)
}

func TestPortalTeamGroupMappingValidateRequiresGroups(t *testing.T) {
	mapping := PortalTeamGroupMappingResource{
		Ref:  "developers-idp-groups",
		Team: "developers",
	}

	err := mapping.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "groups is required")
}
