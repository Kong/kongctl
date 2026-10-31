package planner

import (
	"encoding/base64"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func directoryResource() resources.IdentityDirectoryResource {
	return resources.IdentityDirectoryResource{BaseResource: resources.BaseResource{
		Ref:     "people-ref",
		Kongctl: &resources.KongctlMeta{Namespace: new("identity-team")},
	}, Name: "people"}
}

func TestIdentityDirectoryControlPlaneReferencesAndScope(t *testing.T) {
	for _, existingID := range []string{"", "4078f559-063b-4f10-9098-9e6553b21d9b"} {
		cpAPI := helpers.NewMockControlPlaneAPI(t)
		cpAPI.EXPECT().ListControlPlanes(mock.Anything, mock.Anything).
			Return(newListControlPlaneResponse(nil, 0), nil).Maybe()
		client := state.NewClient(state.ClientConfig{ControlPlaneAPI: cpAPI, IdentityDirectoriesAPI: &fixture.DirectoryAPI{}})
		desired := directoryResource()
		desired.AllowedControlPlanes = []string{"__REF__:gateway#id"}
		cp := resources.ControlPlaneResource{
			BaseResource: resources.BaseResource{
				Ref:     "gateway",
				Kongctl: &resources.KongctlMeta{Namespace: new("identity-team")},
			},
			CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: "gateway"},
		}
		if existingID != "" {
			cp.External = &resources.ExternalBlock{ID: existingID}
		}
		rs := &resources.ResourceSet{
			ControlPlanes: []resources.ControlPlaneResource{cp},
			Identity:      &resources.IdentityResource{Directories: []resources.IdentityDirectoryResource{desired}},
		}
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
		require.NoError(t, err)
		var directory *PlannedChange
		for i := range plan.Changes {
			if plan.Changes[i].ResourceType == ResourceTypeIdentityDirectory {
				directory = &plan.Changes[i]
			}
		}
		require.NotNil(t, directory)
		if existingID != "" {
			require.Equal(t, []any{existingID}, directory.Fields[FieldAllowedControlPlanes])
		} else {
			require.Len(t, directory.DependsOn, 1)
			require.Equal(t, plan.Changes[0].ID, directory.DependsOn[0])
			require.Contains(t, directory.References, FieldAllowedControlPlanes+".0")
		}
	}

	api := &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{fixture.Directory("people", "identity-team", false)}}
	for _, scoped := range []bool{false, true} {
		rs := &resources.ResourceSet{DefaultNamespace: "identity-team", SyncScope: resources.NewSyncScope()}
		if scoped {
			rs.SyncScope.AddRoot(resources.ResourceTypeIdentityDirectory)
		}
		plan, err := NewPlanner(state.NewClient(state.ClientConfig{IdentityDirectoriesAPI: api}), slog.Default()).
			GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
		require.NoError(t, err)
		if scoped {
			require.Len(t, plan.Changes, 1)
		} else {
			require.Empty(t, plan.Changes)
		}
	}
}

func TestIdentityDirectoryControlPlaneLookup(t *testing.T) {
	id := "4078f559-063b-4f10-9098-9e6553b21d9b"
	cpAPI := helpers.NewMockControlPlaneAPI(t)
	cpAPI.EXPECT().ListControlPlanes(mock.Anything, mock.Anything).
		Return(newListControlPlaneResponse([]kkComps.ControlPlane{{ID: id, Name: "gateway"}}, 1), nil)
	desired := directoryResource()
	desired.AllowedControlPlanes = []string{tags.ExternalPlaceholderPrefix +
		base64.RawURLEncoding.EncodeToString([]byte(`{"match_fields":{"name":"gateway"}}`))}
	rs := &resources.ResourceSet{Identity: &resources.IdentityResource{
		Directories: []resources.IdentityDirectoryResource{desired},
	}}
	client := state.NewClient(state.ClientConfig{ControlPlaneAPI: cpAPI, IdentityDirectoriesAPI: &fixture.DirectoryAPI{}})
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, []any{id}, plan.Changes[0].Fields[FieldAllowedControlPlanes])
}

func directoryPlan(t *testing.T, current []kkComps.KongDirectory,
	desired []resources.IdentityDirectoryResource, mode PlanMode,
) (*Plan, error) {
	t.Helper()
	api := &fixture.DirectoryAPI{Directories: current}
	p := NewPlanner(state.NewClient(state.ClientConfig{IdentityDirectoriesAPI: api}), slog.Default())
	p.resources = &resources.ResourceSet{Identity: &resources.IdentityResource{Directories: desired}}
	plan := NewPlan(CurrentPlanVersion, "test", mode)
	directoryPlanner := &identityDirectoryPlanner{BasePlanner: NewBasePlanner(p)}
	err := directoryPlanner.PlanChanges(t.Context(), NewConfig("identity-team"), plan)
	return plan, err
}

func TestIdentityDirectoryLifecycle(t *testing.T) {
	desired := directoryResource()
	current := fixture.Directory("people", "identity-team", false)
	plan, err := directoryPlan(t, nil,
		[]resources.IdentityDirectoryResource{desired}, PlanModeApply)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionCreate, plan.Changes[0].Action)
	require.NotContains(t, plan.Changes[0].Fields, FieldLabels)
	require.Equal(t, resources.BuildChildManagedBy("identity-team", false, "dev"), plan.Changes[0].Fields[FieldManagedBy])
	plan, err = directoryPlan(t, []kkComps.KongDirectory{current},
		[]resources.IdentityDirectoryResource{desired}, PlanModeApply)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	current.ManagedBy[resources.ManagedByVersionKey] = "previous-version"
	plan, err = directoryPlan(t, []kkComps.KongDirectory{current},
		[]resources.IdentityDirectoryResource{desired}, PlanModeApply)
	require.NoError(t, err)
	require.Empty(t, plan.Changes, "version changes alone must not trigger PUTs")

	desired.Description = new("")
	desired.Labels = map[string]string{}
	desired.AllowedControlPlanes = []string{}
	desired.AllowAllControlPlanes = new(false)
	desired.TTLSecs = new(int64(300))
	desired.NegativeTTLSecs = new(int64(86400))
	desired.VaultEnabled = new(true)
	current.AllowedControlPlanes = []string{"gateway-id"}
	plan, err = directoryPlan(t, []kkComps.KongDirectory{current},
		[]resources.IdentityDirectoryResource{desired}, PlanModeApply)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionUpdate, plan.Changes[0].Action)
	for _, field := range []string{
		FieldDescription, FieldLabels, FieldAllowedControlPlanes,
		FieldAllowAllControlPlanes, FieldTTLSecs, FieldNegativeTTLSecs, FieldVaultEnabled,
	} {
		require.Contains(t, plan.Changes[0].ChangedFields, field)
	}
}

func TestIdentityDirectoryOwnershipAndProtection(t *testing.T) {
	for _, namespace := range []string{"", "other-team"} {
		current := fixture.Directory("people", namespace, false)
		_, err := directoryPlan(t, []kkComps.KongDirectory{current},
			[]resources.IdentityDirectoryResource{directoryResource()}, PlanModeApply)
		require.Error(t, err, "a same-name unowned or foreign directory must not be created over or silently adopted")
	}
	protected := fixture.Directory("people", "identity-team", true)
	desired := directoryResource()
	desired.Description = new("changed")
	_, err := directoryPlan(t, []kkComps.KongDirectory{protected},
		[]resources.IdentityDirectoryResource{desired}, PlanModeApply)
	require.ErrorContains(t, err, "protected")
	_, err = directoryPlan(t, []kkComps.KongDirectory{protected}, nil, PlanModeSync)
	require.ErrorContains(t, err, "protected")
	plan, err := directoryPlan(t, []kkComps.KongDirectory{protected},
		[]resources.IdentityDirectoryResource{directoryResource()}, PlanModeApply)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ProtectionChange{Old: true, New: false}, plan.Changes[0].Protection)
	require.Len(t, plan.Changes[0].ChangedFields, 1)
	require.Contains(t, plan.Changes[0].ChangedFields, FieldManagedBy)

	owned := fixture.Directory("people", "identity-team", false)
	foreign := fixture.Directory("foreign", "other-team", false)
	unowned := fixture.Directory("unowned", "", false)
	plan, err = directoryPlan(t, []kkComps.KongDirectory{owned, foreign, unowned}, nil, PlanModeSync)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, owned.ID, plan.Changes[0].ResourceID)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
}

func TestIdentityDirectoryExternalAndDuplicateMatches(t *testing.T) {
	current := fixture.Directory("default", "identity-team", false)
	desired := directoryResource()
	desired.External = &resources.ExternalBlock{ID: current.ID}
	desired.SetKonnectID(current.ID)
	plan, err := directoryPlan(t, []kkComps.KongDirectory{current},
		[]resources.IdentityDirectoryResource{desired}, PlanModeSync)
	require.NoError(t, err)
	require.Empty(t, plan.Changes, "externally declared resources must be excluded from sync deletion")
	_, err = directoryPlan(t, []kkComps.KongDirectory{current, current}, nil, PlanModeSync)
	require.ErrorContains(t, err, "multiple directories")
}
