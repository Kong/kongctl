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

func groupExecutorFixture() (*IdentityGroupAdapter, *fixture.GroupAPI, *ExecutionContext) {
	const directoryID = "4078f559-063b-4f10-9098-9e6553b21d9b"
	current := fixture.Group("Fulfillment", "group-team", false)
	api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{directoryID: {current}}}
	change := planner.PlannedChange{
		ResourceType: planner.ResourceTypeIdentityGroup, ResourceRef: "fulfillment", ResourceID: current.ID,
		Action: planner.ActionUpdate, Namespace: "group-team", Parent: &planner.ParentInfo{ID: directoryID},
		Fields: map[string]any{
			planner.FieldName: current.Name, planner.FieldDescription: "Changed",
			planner.FieldManagedBy: resources.BuildChildManagedBy("group-team", false, ""),
		},
	}
	return &IdentityGroupAdapter{
			client: state.NewClient(
				state.ClientConfig{
					IdentityBasicAuthAPI:        &fixture.BasicAuthAPI{},
					IdentityPrincipalKeysReader: &fixture.BasicAuthAPI{},
					IdentityGroupsAPI:           api,
				},
			),
		}, api, NewExecutionContext(
			&change,
		)
}

func TestGroupSavedPutPreservesOmissionAndClearsExplicitMaps(t *testing.T) {
	a, api, ctx := groupExecutorFixture()
	data, err := json.Marshal(ctx.PlannedChange)
	require.NoError(t, err)
	var saved planner.PlannedChange
	require.NoError(t, json.Unmarshal(data, &saved))
	ctx = NewExecutionContext(&saved)
	var request kkComps.ReplaceGroupBody
	require.NoError(t, a.MapUpdateFields(t.Context(), ctx, saved.Fields, &request, nil))
	_, err = a.Update(t.Context(), saved.ResourceID, request, ctx.Namespace, ctx)
	require.NoError(t, err)
	require.Len(t, api.Writes, 1)
	require.Equal(t, map[string]string{"team": "identity"}, api.Writes[0].Labels)
	require.Equal(t, int64(42), *api.Writes[0].Metadata["count"].Integer)
	require.Len(t, api.Writes[0].ManagedBy, 4)
	ctx.PlannedChange.Fields[planner.FieldLabels] = map[string]string{}
	ctx.PlannedChange.Fields[planner.FieldMetadata] = map[string]any{}
	request = kkComps.ReplaceGroupBody{}
	require.NoError(t, a.MapUpdateFields(t.Context(), ctx, ctx.PlannedChange.Fields, &request, nil))
	_, err = a.Update(t.Context(), saved.ResourceID, request, ctx.Namespace, ctx)
	require.NoError(t, err)
	require.Empty(t, api.Writes[1].Labels)
	require.Empty(t, api.Writes[1].Metadata)
}

func TestGroupSavedMutationsRecheckFreshAuthorityAndDescendants(t *testing.T) {
	for _, fault := range []string{
		"foreign", "protected", "renamed", "team", "unknown-provenance", "duplicate", "keys", "membership",
	} {
		t.Run(fault, func(t *testing.T) {
			a, api, ctx := groupExecutorFixture()
			id := ctx.PlannedChange.ResourceID
			directory := ctx.PlannedChange.Parent.ID
			switch fault {
			case "foreign":
				api.Directories[directory][0].ManagedBy[resources.ChildNamespaceKey] = "other"
			case "protected":
				api.Directories[directory][0].ManagedBy[resources.ChildProtectedKey] = "true"
			case "renamed":
				api.Directories[directory][0].Name = "Other"
			case "team":
				api.Directories[directory][0].Type = new(kkComps.KongGroupTypeTeam)
			case "unknown-provenance":
				api.Directories[directory][0].ProvisionedBy = nil
			case "duplicate":
				other := api.Directories[directory][0]
				other.ID = "5078f559-063b-4f10-9098-9e6553b21d9b"
				api.Directories[directory] = append(api.Directories[directory], other)
			case "keys":
				api.Keys = map[string][]string{id: {"late-key"}}
			case "membership":
				api.Failure = "membership"
			}
			ctx.PlannedChange.Action = planner.ActionDelete
			err := a.Delete(t.Context(), id, ctx)
			require.Error(t, err)
			require.Zero(t, api.Deletes)
			if fault != "keys" && fault != "membership" {
				ctx.PlannedChange.Action = planner.ActionUpdate
				var request kkComps.ReplaceGroupBody
				require.NoError(t, a.MapUpdateFields(t.Context(), ctx, ctx.PlannedChange.Fields, &request, nil))
				_, err = a.Update(t.Context(), id, request, ctx.Namespace, ctx)
				require.Error(t, err)
				require.Empty(t, api.Writes)
			}
		})
	}
}

func TestGroupSavedFieldsRejectUnsupportedAndRoundedValues(t *testing.T) {
	for _, fields := range []map[string]any{
		{planner.FieldName: "Bad-Name"},
		{planner.FieldName: "Valid", planner.FieldType: "team"},
		{planner.FieldName: "Valid", planner.FieldMetadata: map[string]any{"number": json.Number("9007199254740993")}},
		{planner.FieldName: "Valid", planner.FieldDescription: nil},
	} {
		var create kkComps.CreateGroupBody
		var replace kkComps.ReplaceGroupBody
		require.Error(t, mapIdentityGroupFields(fields, &create))
		require.Error(t, mapIdentityGroupFields(fields, &replace))
	}
}
