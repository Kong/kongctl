package executor

import (
	"encoding/json"
	"fmt"
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
			"large": kkComps.CreateKongIdentityMetadataInteger(9007199254740994),
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
	require.Equal(t, int64(9007199254740994), *api.Writes[0].Metadata["large"].Integer)
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
		Fields:       map[string]any{planner.FieldMetadata: map[string]any{"large": int64(9007199254740994)}},
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
	require.Equal(t, int64(9007199254740994), *request.Metadata["large"].Integer)
}

func TestPrincipalSavedWritesRejectMetadataRoundingBeforeSDKMapping(t *testing.T) {
	for _, number := range []string{"9007199254740993", "-9007199254740993", "1152921504606846976"} {
		change := planner.PlannedChange{
			ResourceType: planner.ResourceTypeIdentityPrincipal,
			Fields:       map[string]any{planner.FieldMetadata: map[string]any{"identifier": json.Number(number)}},
		}
		encoded, err := json.Marshal(change)
		require.NoError(t, err)
		var saved planner.PlannedChange
		require.NoError(t, json.Unmarshal(encoded, &saved))
		execution, err := cloneChangeForExecution(&saved)
		require.NoError(t, err)
		var create kkComps.CreatePrincipalBody
		require.ErrorContains(t, mapIdentityPrincipalFields(execution.Fields, &create), "Q31")
		var update kkComps.ReplacePrincipalBody
		require.ErrorContains(t, mapIdentityPrincipalFields(execution.Fields, &update), "Q31")
	}
}

func TestPrincipalSavedDeleteRechecksIdentityChildren(t *testing.T) {
	for _, mode := range []string{"empty", "unowned", "owned-protected", "foreign", "failed", "unconfigured"} {
		t.Run(mode, func(t *testing.T) {
			const directoryID = "731559d3-39bd-48c7-b33c-eb192847eb20"
			const principalID = "731559d3-39bd-48c7-b33c-eb192847eb21"
			a, api, execCtx := principalExecutorFixture()
			current := api.Directories["directory-id"][0]
			current.ID = principalID
			api.Directories = map[string][]kkComps.KongPrincipal{directoryID: {current}}
			execCtx.PlannedChange.Action = planner.ActionDelete
			execCtx.PlannedChange.ResourceID = principalID
			execCtx.PlannedChange.Parent.ID = directoryID
			identities := &fixture.PrincipalIdentitiesAPI{}
			if mode == "failed" {
				identities.Error = fmt.Errorf("incomplete identity observation")
			} else if mode != "empty" && mode != "unconfigured" {
				var metadata map[string]string
				if mode != "unowned" {
					metadata = resources.BuildPrincipalManagedBy("principal-team", "employee", true, "test")
				}
				if mode == "foreign" {
					metadata[resources.ChildNamespaceKey] = "other-team"
				}
				identities.Data = []kkComps.KongPrincipalIdentity{
					kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
						ID: "731559d3-39bd-48c7-b33c-eb192847eb22", Key: "employee", Value: "123", ManagedBy: metadata,
					}),
				}
			}
			config := state.ClientConfig{IdentityPrincipalsAPI: api}
			if mode != "unconfigured" {
				config.IdentityPrincipalIdentitiesAPI = identities
			}
			a.client = state.NewClient(config)
			// This represents a child added after a saved deletion plan. Only a
			// fresh empty collection permits the final parent mutation.
			err := a.Delete(t.Context(), principalID, execCtx)
			if mode == "empty" {
				require.NoError(t, err)
				require.Equal(t, 1, api.Deletes)
			} else {
				require.Error(t, err)
				require.Zero(t, api.Deletes)
				if mode == "failed" || mode == "unconfigured" {
					require.ErrorContains(t, err, "cannot verify principal identities")
				} else {
					require.ErrorContains(t, err, "still has 1 identities")
				}
			}
			if mode != "unconfigured" {
				require.Len(t, identities.Requests, 1)
				require.Equal(t, directoryID, identities.Requests[0].DirectoryID)
				require.Equal(t, principalID, identities.Requests[0].PrincipalID)
			}
		})
	}
}
