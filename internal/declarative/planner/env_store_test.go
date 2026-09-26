package planner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestStoredEnvDiffPreservesTypesAndApprovedValues(t *testing.T) {
	t.Setenv("STORED_DIFF_DESCRIPTION", "approved plaintext")
	t.Setenv("STORED_DIFF_BOOL", "true")
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`portals:
  - ref: stored
    name: stored
    description: !env_store STORED_DIFF_DESCRIPTION
    auto_approve_developers: !env_store STORED_DIFF_BOOL
`), 0o600))
	rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	p := &portalPlannerImpl{BasePlanner: &BasePlanner{}}
	current := state.Portal{ListPortalsResponsePortal: kkComps.ListPortalsResponsePortal{
		Name: "stored", Description: new("old"), AutoApproveDevelopers: new(false),
	}}
	changed, fields, changes := p.shouldUpdatePortal(current, rs.Portals[0])
	require.True(t, changed)
	require.Equal(t, "approved plaintext", fields[FieldDescription])
	require.Equal(t, true, fields[FieldAutoApproveDevelopers])
	require.Equal(t, FieldChange{Old: "old", New: "approved plaintext"}, changes[FieldDescription])
	plan := NewPlan("1.0", "test", PlanModeApply)
	plan.Changes = []PlannedChange{{ResourceRef: "stored", Fields: fields, ChangedFields: changes}}
	(&Planner{}).applyDeferredEnvPlaceholders(plan, rs)
	encoded, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "__ENV__:")
	var restored Plan
	require.NoError(t, json.Unmarshal(encoded, &restored))
	require.Equal(t, changes, restored.Changes[0].ChangedFields)
	current.Description = new("approved plaintext")
	current.AutoApproveDevelopers = new(true)
	current.AuthenticationEnabled = rs.Portals[0].AuthenticationEnabled
	current.AutoApproveApplications = rs.Portals[0].AutoApproveApplications
	current.RbacEnabled = rs.Portals[0].RbacEnabled
	changed, _, changes = p.shouldUpdatePortal(current, rs.Portals[0])
	require.Falsef(t, changed, "unexpected changes: %#v", changes)
}
