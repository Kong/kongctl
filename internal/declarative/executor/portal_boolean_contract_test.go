package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

// Inspect mapped pointers without the SDK's separate wire-default marshaler.
type (
	portalBooleanCreateProjection kkComps.CreatePortal
	portalBooleanUpdateProjection kkComps.UpdatePortal
)

func TestPortalBooleanContractRequests(t *testing.T) {
	fields := []string{
		planner.FieldAuthenticationEnabled, planner.FieldRBACEnabled, planner.FieldSIPREnabled,
		planner.FieldAutoApproveDevelopers, planner.FieldAutoApproveApplications,
	}
	for _, field := range fields {
		for _, tc := range []struct {
			name    string
			value   any
			present bool
		}{
			{"omitted", nil, false},
			{"null", nil, true},
			{"false", false, true},
			{"true", true, true},
			{"string", "false", true},
			{"number", 0, true},
		} {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				for _, seeded := range []bool{false, true} {
					create := kkComps.CreatePortal{}
					update := kkComps.UpdatePortal{}
					var expected any
					if seeded {
						data, err := json.Marshal(map[string]any{field: true})
						require.NoError(t, err)
						var seededCreate portalBooleanCreateProjection
						var seededUpdate portalBooleanUpdateProjection
						require.NoError(t, json.Unmarshal(data, &seededCreate))
						require.NoError(t, json.Unmarshal(data, &seededUpdate))
						create = kkComps.CreatePortal(seededCreate)
						update = kkComps.UpdatePortal(seededUpdate)
						expected = true
					}
					input := map[string]any{planner.FieldName: "portal"}
					if tc.present {
						input[field] = tc.value
					}
					if value, ok := tc.value.(bool); ok {
						expected = value
					}
					adapter := NewPortalAdapter(nil)
					require.NoError(t, adapter.MapCreateFields(t.Context(), &ExecutionContext{}, input, &create))
					require.NoError(t, adapter.MapUpdateFields(t.Context(), &ExecutionContext{}, input, &update, nil))
					for _, request := range []any{portalBooleanCreateProjection(create), portalBooleanUpdateProjection(update)} {
						data, err := json.Marshal(request)
						require.NoError(t, err)
						var actual map[string]any
						require.NoError(t, json.Unmarshal(data, &actual))
						require.Equal(t, expected, actual[field])
						for _, other := range fields {
							if other != field {
								require.Nil(t, actual[other])
							}
						}
					}
				}
			})
		}
	}
}
