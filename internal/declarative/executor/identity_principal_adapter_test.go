package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func principalExecutorFixture() (*IdentityPrincipalAdapter, *fixture.PrincipalAPI, *ExecutionContext) {
	api := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{"directory-id": {{
		ID: "principal-id", DisplayName: new("Current name"), Description: "Fresh description",
		Type: new(kkComps.KongPrincipalTypeUser), Labels: map[string]string{"owner": "team"},
		Metadata: map[string]kkComps.KongIdentityMetadata{
			"large": kkComps.CreateKongIdentityMetadataInteger(9007199254740993),
		},
		ManagedBy: resources.BuildPrincipalManagedBy("principal-team", "alice", false, "old"),
	}}}}
	change := planner.PlannedChange{
		ResourceType: planner.ResourceTypeIdentityPrincipal, ResourceRef: "alice", ResourceID: "principal-id",
		Action: planner.ActionUpdate, Namespace: "principal-team",
		Parent: &planner.ParentInfo{ID: "directory-id"}, Fields: map[string]any{
			planner.FieldDisplayName: "Updated name",
			planner.FieldManagedBy:   resources.BuildPrincipalManagedBy("principal-team", "alice", false, ""),
		},
	}
	return &IdentityPrincipalAdapter{
			client: state.NewClient(state.ClientConfig{IdentityPrincipalsAPI: api}),
		}, api, NewExecutionContext(
			&change,
		)
}

func TestPrincipalSavedPutPreservesFreshOmittedFields(t *testing.T) {
	a, api, execCtx := principalExecutorFixture()
	encoded, err := json.Marshal(execCtx.PlannedChange)
	require.NoError(t, err)
	var saved planner.PlannedChange
	require.NoError(t, json.Unmarshal(encoded, &saved))
	execCtx = NewExecutionContext(&saved)
	var request kkComps.ReplacePrincipalBody
	require.NoError(t, a.MapUpdateFields(t.Context(), execCtx, execCtx.PlannedChange.Fields, &request, nil))
	_, err = a.Update(t.Context(), "principal-id", request, execCtx.Namespace, execCtx)
	require.NoError(t, err)
	require.Len(t, api.Writes, 1)
	require.Equal(t, "Fresh description", *api.Writes[0].Description)
	require.Equal(t, kkComps.KongPrincipalTypeUser, *api.Writes[0].Type)
	require.Equal(t, map[string]string{"owner": "team"}, api.Writes[0].Labels)
	require.Nil(t, api.Writes[0].Active, "Q10: never synthesize false")
	require.Equal(t, int64(9007199254740993), *api.Writes[0].Metadata["large"].Integer)
	require.Equal(t, "alice", api.Writes[0].ManagedBy[resources.ManagedByRefKey])
	for _, field := range []string{planner.FieldLabels, planner.FieldMetadata} {
		a, api, execCtx := principalExecutorFixture()
		execCtx.PlannedChange.Fields[field] = map[string]any{}
		require.NoError(t, a.MapUpdateFields(t.Context(), execCtx, execCtx.PlannedChange.Fields, &request, nil))
		_, err := a.Update(t.Context(), "principal-id", request, execCtx.Namespace, execCtx)
		require.NoError(t, err)
		require.Len(t, api.Writes, 1)
		if field == planner.FieldLabels {
			require.Empty(t, api.Writes[0].Labels)
		} else {
			require.Empty(t, api.Writes[0].Metadata)
		}
	}
}

func TestPrincipalSavedMutationRejectsNewDuplicate(t *testing.T) {
	for _, action := range []planner.ActionType{planner.ActionUpdate, planner.ActionDelete} {
		a, api, execCtx := principalExecutorFixture()
		duplicate := api.Directories["directory-id"][0]
		duplicate.ID = "duplicate-id"
		api.Directories["directory-id"] = append(api.Directories["directory-id"], duplicate)
		var err error
		if action == planner.ActionDelete {
			err = a.Delete(t.Context(), "principal-id", execCtx)
		} else {
			var request kkComps.ReplacePrincipalBody
			require.NoError(t, a.MapUpdateFields(t.Context(), execCtx, execCtx.PlannedChange.Fields, &request, nil))
			_, err = a.Update(t.Context(), "principal-id", request, execCtx.Namespace, execCtx)
		}
		require.ErrorContains(t, err, "multiple principals")
		require.Empty(t, api.Writes)
		require.Zero(t, api.Deletes)
	}
}

func TestPrincipalRejectsChangedOwnershipTypeAndProtection(t *testing.T) {
	for _, field := range []string{
		resources.ManagedByToolKey, resources.ChildNamespaceKey,
		resources.ManagedByRefKey, resources.ChildProtectedKey, planner.FieldType,
	} {
		t.Run(field, func(t *testing.T) {
			a, api, execCtx := principalExecutorFixture()
			switch field {
			case planner.FieldType:
				execCtx.PlannedChange.Fields[field] = "agent"
			case resources.ChildProtectedKey:
				api.Directories["directory-id"][0].ManagedBy[field] = "true"
			default:
				api.Directories["directory-id"][0].ManagedBy[field] = "other"
			}
			var request kkComps.ReplacePrincipalBody
			require.NoError(t, a.MapUpdateFields(t.Context(), execCtx, execCtx.PlannedChange.Fields, &request, nil))
			_, err := a.Update(t.Context(), "principal-id", request, execCtx.Namespace, execCtx)
			require.Error(t, err)
			require.Empty(t, api.Writes)
		})
	}
	a, _, execCtx := principalExecutorFixture()
	var request kkComps.ReplacePrincipalBody
	require.ErrorContains(
		t,
		a.MapUpdateFields(t.Context(), execCtx, map[string]any{"active": false}, &request, nil),
		"Q10",
	)
}

func TestPrincipalSavedCreateRetainsIntegerMetadata(t *testing.T) {
	change := planner.PlannedChange{
		ResourceType: planner.ResourceTypeIdentityPrincipal,
		Fields:       map[string]any{planner.FieldMetadata: map[string]any{"large": int64(9007199254740993)}},
	}
	encoded, err := json.Marshal(change)
	require.NoError(t, err)
	var saved planner.PlannedChange
	require.NoError(t, json.Unmarshal(encoded, &saved))
	var request kkComps.CreatePrincipalBody
	execution, err := cloneChangeForExecution(&saved)
	require.NoError(t, err)
	require.NoError(t, (&Executor{}).resolveDeferredEnvPlaceholders(execution))
	require.NoError(t, mapIdentityPrincipalFields(execution.Fields, &request))
	require.Equal(t, int64(9007199254740993), *request.Metadata["large"].Integer)
}
