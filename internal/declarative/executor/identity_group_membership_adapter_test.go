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

func membershipExecutorFixture() (*identityGroupMembershipAdapter, *fixture.GroupAPI, *ExecutionContext) {
	const directoryID = "4078f559-063b-4f10-9098-9e6553b21d9b"
	const principalID = "5078f559-063b-4f10-9098-9e6553b21d9b"
	group := fixture.Group("Operators", "group-team", false)
	api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{directoryID: {group}}}
	client := state.NewClient(state.ClientConfig{
		IdentityBasicAuthAPI:        &fixture.BasicAuthAPI{},
		IdentityPrincipalKeysReader: &fixture.BasicAuthAPI{},
		IdentityGroupsAPI:           api,
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{
			Directories: map[string][]kkComps.KongPrincipal{directoryID: {{ID: principalID}}},
		},
	})
	change := planner.PlannedChange{
		ResourceType: planner.ResourceTypeIdentityGroupMembership, ResourceRef: "alice-member", Action: planner.ActionCreate,
		Namespace: "group-team", Protection: false,
		Parent:     &planner.ParentInfo{Ref: "operators", ID: group.ID},
		References: map[string]planner.ReferenceInfo{planner.FieldDirectoryID: {ID: directoryID}},
		ProtectingParent: &planner.ProtectingParentInfo{
			ResourceType: planner.ResourceTypeIdentityGroup,
			ResourceName: group.Name,
			ResourceID:   group.ID,
		},
		Fields: map[string]any{
			planner.FieldMember: map[string]any{planner.FieldType: "principal", planner.FieldID: principalID},
		},
	}
	return &identityGroupMembershipAdapter{client: client}, api, NewExecutionContext(&change)
}

func TestSavedMembershipRoutesByEntryAndPreservesPrincipalAndGroup(t *testing.T) {
	a, api, ctx := membershipExecutorFixture()
	data, err := json.Marshal(ctx.PlannedChange)
	require.NoError(t, err)
	var saved planner.PlannedChange
	require.NoError(t, json.Unmarshal(data, &saved))
	ctx = NewExecutionContext(&saved)
	var request kkComps.CreateGroupMembershipEntryBody
	require.NoError(t, a.MapCreateFields(t.Context(), ctx, saved.Fields, &request))
	id, err := a.Create(t.Context(), request, ctx.Namespace, ctx)
	require.NoError(t, err)
	require.NotEqual(t, request.Member.ID, id)
	require.Equal(t, 1, api.MembershipCreates)
	ctx.PlannedChange.Action = planner.ActionDelete
	ctx.PlannedChange.ResourceID = id
	require.NoError(t, a.Delete(t.Context(), id, ctx))
	require.Equal(t, 1, api.MembershipDeletes)
	require.Empty(t, api.Memberships[ctx.PlannedChange.Parent.ID])
	require.NoError(t, a.Delete(t.Context(), id, ctx))
	require.Equal(t, 1, api.MembershipDeletes)
	require.Zero(t, api.Deletes)
	require.Empty(t, api.Writes)
}

func TestMembershipSavedWritesRepeatGroupAndMemberAuthority(t *testing.T) {
	for _, fault := range []string{
		"foreign", "unowned", "renamed", "team", "provenance", "principal-scope", "incomplete",
	} {
		t.Run(fault, func(t *testing.T) {
			a, api, ctx := membershipExecutorFixture()
			directoryID := ctx.PlannedChange.References[planner.FieldDirectoryID].ID
			group := &api.Directories[directoryID][0]
			switch fault {
			case "foreign":
				group.ManagedBy[resources.ChildNamespaceKey] = "foreign"
			case "unowned":
				group.ManagedBy = nil
			case "renamed":
				group.Name = "Other"
			case "team":
				group.Type = new(kkComps.KongGroupTypeTeam)
			case "provenance":
				group.ProvisionedBy = nil
			case "principal-scope":
				member := ctx.PlannedChange.Fields[planner.FieldMember].(map[string]any)
				member[planner.FieldID] = "7078f559-063b-4f10-9098-9e6553b21d9b"
			case "incomplete":
				api.Failure = "membership"
			}
			var request kkComps.CreateGroupMembershipEntryBody
			require.NoError(t, a.MapCreateFields(t.Context(), ctx, ctx.PlannedChange.Fields, &request))
			_, err := a.Create(t.Context(), request, ctx.Namespace, ctx)
			require.Error(t, err)
			require.Zero(t, api.MembershipCreates)
		})
	}
}

func TestMembershipDeleteRejectsProtectionAndChangedMember(t *testing.T) {
	for _, fault := range []string{"protected", "changed-member", "wrong-entry"} {
		t.Run(fault, func(t *testing.T) {
			a, api, ctx := membershipExecutorFixture()
			var request kkComps.CreateGroupMembershipEntryBody
			require.NoError(t, a.MapCreateFields(t.Context(), ctx, ctx.PlannedChange.Fields, &request))
			id, err := a.Create(t.Context(), request, ctx.Namespace, ctx)
			require.NoError(t, err)
			ctx.PlannedChange.Action = planner.ActionDelete
			switch fault {
			case "protected":
				directoryID := ctx.PlannedChange.References[planner.FieldDirectoryID].ID
				api.Directories[directoryID][0].ManagedBy[resources.ChildProtectedKey] = "true"
			case "changed-member":
				member := ctx.PlannedChange.Fields[planner.FieldMember].(map[string]any)
				member[planner.FieldID] = "7078f559-063b-4f10-9098-9e6553b21d9b"
			case "wrong-entry":
				// The principal UUID cannot select a membership entry.
				require.NoError(t, a.Delete(t.Context(), request.Member.ID, ctx))
				require.Zero(t, api.MembershipDeletes)
				return
			}
			require.Error(t, a.Delete(t.Context(), id, ctx))
			require.Zero(t, api.MembershipDeletes)
		})
	}
}

func TestMembershipPayloadRejectsNestedAndUnknownFields(t *testing.T) {
	for _, fields := range []map[string]any{
		{planner.FieldMember: map[string]any{
			planner.FieldType: "group", planner.FieldID: "5078f559-063b-4f10-9098-9e6553b21d9b",
		}},
		{planner.FieldMember: map[string]any{planner.FieldType: "principal", planner.FieldID: "invalid"}},
		{planner.FieldMember: map[string]any{
			planner.FieldType: "principal", planner.FieldID: "5078f559-063b-4f10-9098-9e6553b21d9b",
			"unexpected": true,
		}},
		{planner.FieldMember: nil},
	} {
		var request kkComps.CreateGroupMembershipEntryBody
		require.Error(t, mapIdentityMembership(fields, &request))
	}
}
