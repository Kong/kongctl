package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func basicAuthExecutorFixture() (*IdentityBasicAuthAdapter, *fixture.BasicAuthAPI, *ExecutionContext) {
	const directory, principal = "4078f559-063b-4f10-9098-9e6553b21d9b", "5078f559-063b-4f10-9098-9e6553b21d9b"
	current := fixture.BasicAuth("worker", "credentials", false)
	api := &fixture.BasicAuthAPI{
		Parents: map[string][]helpers.IdentityBasicAuth{directory + "/" + principal: {current}},
	}
	change := planner.PlannedChange{
		ResourceType: planner.ResourceTypeIdentityBasicAuth, ResourceRef: "login", ResourceID: current.ID,
		Action: planner.ActionUpdate, Namespace: "credentials", Parent: &planner.ParentInfo{ID: principal},
		References: map[string]planner.ReferenceInfo{planner.FieldDirectoryID: {ID: directory}},
		Fields: map[string]any{
			planner.FieldUsername:  current.Username,
			planner.FieldManagedBy: resources.BuildChildManagedBy("credentials", false, ""),
		},
	}
	return &IdentityBasicAuthAdapter{client: state.NewClient(state.ClientConfig{IdentityBasicAuthAPI: api})},
		api, NewExecutionContext(&change)
}

func TestBasicAuthSavedUpdateChecksIdentityAndMetadata(t *testing.T) {
	a, api, ctx := basicAuthExecutorFixture()
	data, err := json.Marshal(ctx.PlannedChange)
	require.NoError(t, err)
	var saved planner.PlannedChange
	require.NoError(t, json.Unmarshal(data, &saved))
	ctx = NewExecutionContext(&saved)
	var request kkComps.ReplaceBasicAuthBody
	require.NoError(t, a.MapUpdateFields(t.Context(), ctx, saved.Fields, &request, nil))
	_, err = a.Update(t.Context(), saved.ResourceID, request, ctx.Namespace, ctx)
	require.NoError(t, err)
	require.Len(t, api.Writes, 1)
	require.Equal(t, "worker", api.Writes[0].Username)
	require.Len(t, api.Writes[0].ManagedBy, 4)
	for _, field := range []string{"secret", "passwords", planner.FieldLabels} {
		saved.Fields[field] = "forged"
		require.Error(t, a.MapUpdateFields(t.Context(), ctx, saved.Fields, &request, nil))
		delete(saved.Fields, field)
	}
}

func TestBasicAuthSavedMutationsRecheckAuthorityAndPasswordDescendants(t *testing.T) {
	for _, fault := range []string{"foreign", "protected", "renamed", "duplicate", "password", "password-list", "list"} {
		t.Run(fault, func(t *testing.T) {
			a, api, ctx := basicAuthExecutorFixture()
			key := ctx.PlannedChange.References[planner.FieldDirectoryID].ID + "/" + ctx.PlannedChange.Parent.ID
			id := ctx.PlannedChange.ResourceID
			switch fault {
			case "foreign":
				api.Parents[key][0].ManagedBy[resources.ChildNamespaceKey] = "other"
			case "protected":
				api.Parents[key][0].ManagedBy[resources.ChildProtectedKey] = "true"
			case "renamed":
				api.Parents[key][0].Username = "other"
			case "duplicate":
				other := api.Parents[key][0]
				other.ID = ctx.PlannedChange.Parent.ID
				api.Parents[key] = append(api.Parents[key], other)
			case "password":
				api.Passwords = map[string][]helpers.IdentityPassword{
					key + "/" + id: {{ID: ctx.PlannedChange.Parent.ID}},
				}
			default:
				api.Failure = fault
			}
			ctx.PlannedChange.Action = planner.ActionDelete
			require.Error(t, a.Delete(t.Context(), id, ctx))
			require.Zero(t, api.Deletes)
			if fault != "password" && fault != "password-list" {
				ctx.PlannedChange.Action = planner.ActionUpdate
				var request kkComps.ReplaceBasicAuthBody
				require.NoError(t, a.MapUpdateFields(t.Context(), ctx, ctx.PlannedChange.Fields, &request, nil))
				_, err := a.Update(t.Context(), id, request, ctx.Namespace, ctx)
				require.Error(t, err)
				require.Empty(t, api.Writes)
			}
		})
	}
}
